// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/ava-labs/avalanchego/api/health"
	avacache "github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/statesync"
)

var (
	_ block.ChainVM         = (*VM[Block, Block, Block])(nil)
	_ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)
)

type ChainInput struct {
	SnowCtx                                 *snow.Context
	GenesisBytes, UpgradeBytes, ConfigBytes []byte
	ToEngine                                chan<- common.Message
	Shutdown                                <-chan struct{}
	Tracer                                  trace.Tracer
}

type Chain[I Block, O Block, A Block] interface {
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		chainIndex ChainIndex[I, O, A],
		options *Options[I, O, A],
	) (I, O, A, error)
	BuildBlock(ctx context.Context, parent O) (I, O, error)
	ParseBlock(ctx context.Context, bytes []byte) (I, error)
	Execute(
		ctx context.Context,
		parent O,
		block I,
	) (O, error)
	AcceptBlock(ctx context.Context, verifiedBlock O) (A, error)
	AcceptDynamicStateSyncBlock(ctx context.Context, block I) error
	GetBlock(ctx context.Context, blkID ids.ID) ([]byte, error)
	GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error)
}

type VM[I Block, O Block, A Block] struct {
	chain       Chain[I, O, A]
	covariantVM *CovariantVM[I, O, A]
	Options     Options[I, O, A]

	snowCtx *snow.Context

	config   Config
	vmConfig VMConfig
	// We cannot use a map here because we may parse blocks up in the ancestry
	parsedBlocks *avacache.LRU[ids.ID, *StatefulBlock[I, O, A]]

	// Each element is a block that passed verification but
	// hasn't yet been accepted/rejected
	verifiedL      sync.RWMutex
	verifiedBlocks map[ids.ID]*StatefulBlock[I, O, A]

	// We store the last [AcceptedBlockWindowCache] blocks in memory
	// to avoid reading blocks from disk.
	acceptedBlocksByID     *cache.FIFO[ids.ID, *StatefulBlock[I, O, A]]
	acceptedBlocksByHeight *cache.FIFO[uint64, ids.ID]

	lastAcceptedBlock *StatefulBlock[I, O, A]
	preferredBlkID    ids.ID

	metrics *Metrics
	log     logging.Logger
	tracer  trace.Tracer

	shutdownChan chan struct{}
}

type Options[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]

	Version         string
	Handlers        map[string]http.Handler
	HealthChecker   health.Checker
	Network         *p2p.Network
	StateSyncClient *statesync.Client[*StatefulBlock[I, O, A]]
	StateSyncServer *statesync.Server[*StatefulBlock[I, O, A]]
	Closers         []func() error

	VerifiedSubs []event.Subscription[O]
	RejectedSubs []event.Subscription[O]
	AcceptedSubs []event.Subscription[A]
}

type Option[I Block, O Block, A Block] func(*Options[I, O, A])

func (o *Options[I, O, A]) WithAcceptedSub(sub ...event.Subscription[A]) {
	o.AcceptedSubs = append(o.AcceptedSubs, sub...)
}

func (o *Options[I, O, A]) WithRejectedSub(sub ...event.Subscription[O]) {
	o.RejectedSubs = append(o.RejectedSubs, sub...)
}

func (o *Options[I, O, A]) WithVerifiedSub(sub ...event.Subscription[O]) {
	o.VerifiedSubs = append(o.VerifiedSubs, sub...)
}

func NewVM[I Block, O Block, A Block](chain Chain[I, O, A]) *VM[I, O, A] {
	return &VM[I, O, A]{
		chain: chain,
		Options: Options[I, O, A]{
			Version: "v0.0.1",
			HealthChecker: health.CheckerFunc(func(ctx context.Context) (interface{}, error) {
				return nil, nil
			}),
		},
	}
}

func (v *VM[I, O, A]) Initialize(
	ctx context.Context,
	chainCtx *snow.Context,
	db database.Database,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender,
) error {
	v.covariantVM = &CovariantVM[I, O, A]{v}
	v.shutdownChan = make(chan struct{})
	config, err := NewConfig(configBytes)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	v.config = config
	v.vmConfig, err = GetConfig[VMConfig](config, "vm", NewDefaultVMConfig())
	if err != nil {
		return fmt.Errorf("failed to parse vm config: %w", err)
	}
	defaultRegistry, metrics, err := newMetrics()
	if err != nil {
		return err
	}
	if err := chainCtx.Metrics.Register("hypersdk", defaultRegistry); err != nil {
		return err
	}
	v.metrics = metrics
	v.log = chainCtx.Log

	// Setup tracer
	traceConfig, err := GetConfig[trace.Config](v.config, "tracer", trace.Config{Enabled: false})
	if err != nil {
		return err
	}
	v.tracer, err = trace.New(traceConfig)
	if err != nil {
		return err
	}
	ctx, span := v.tracer.Start(ctx, "VM.Initialize")
	defer span.End()

	continuousProfilerConfig, err := GetConfig[profiler.Config](v.config, "continuousProfiler", profiler.Config{
		Enabled: false,
	})
	if err != nil {
		return fmt.Errorf("failed to parse continuous profiler config: %w", err)
	}
	if continuousProfilerConfig.Enabled {
		continuousProfiler := profiler.NewContinuous(
			continuousProfilerConfig.Dir,
			continuousProfilerConfig.Freq,
			continuousProfilerConfig.MaxNumFiles,
		)
		v.Options.WithCloser(func() error {
			continuousProfiler.Shutdown()
			return nil
		})
		go continuousProfiler.Dispatch() //nolint:errcheck
	}

	v.Options.Network, err = p2p.NewNetwork(v.log, appSender, defaultRegistry, "p2p")
	if err != nil {
		return fmt.Errorf("failed to initialize p2p: %w", err)
	}

	acceptedBlocksByIDCache, err := cache.NewFIFO[ids.ID, *StatefulBlock[I, O, A]](v.vmConfig.AcceptedBlockWindowCache)
	if err != nil {
		return err
	}
	v.acceptedBlocksByID = acceptedBlocksByIDCache
	acceptedBlocksByHeightCache, err := cache.NewFIFO[uint64, ids.ID](v.vmConfig.AcceptedBlockWindowCache)
	if err != nil {
		return err
	}
	v.acceptedBlocksByHeight = acceptedBlocksByHeightCache
	v.parsedBlocks = &avacache.LRU[ids.ID, *StatefulBlock[I, O, A]]{Size: v.vmConfig.ParsedBlockCacheSize}
	v.verifiedBlocks = make(map[ids.ID]*StatefulBlock[I, O, A])

	chainInput := ChainInput{
		SnowCtx:      chainCtx,
		GenesisBytes: genesisBytes,
		UpgradeBytes: upgradeBytes,
		ConfigBytes:  configBytes,
		ToEngine:     toEngine,
		Shutdown:     v.shutdownChan,
		Tracer:       v.tracer,
	}

	inputBlock, outputBlock, acceptedBlock, err := v.chain.Initialize(ctx, chainInput, ChainIndex[I, O, A]{
		CovariantVM: v.covariantVM,
	}, &v.Options)
	if err != nil {
		return err
	}
	v.lastAcceptedBlock = NewAcceptedBlock(v.covariantVM, inputBlock, outputBlock, acceptedBlock)
	v.preferredBlkID = v.lastAcceptedBlock.ID()
	v.acceptedBlocksByHeight.Put(v.lastAcceptedBlock.Height(), v.lastAcceptedBlock.ID())
	v.acceptedBlocksByID.Put(v.lastAcceptedBlock.ID(), v.lastAcceptedBlock)

	return nil
}

// TODO
func (v *VM[I, O, A]) SetState(ctx context.Context, state snow.State) error {
	return nil
}

func WithHealthChecker[I Block, O Block, A Block](healthChecker health.Checker) Option[I, O, A] {
	return func(opts *Options[I, O, A]) {
		opts.HealthChecker = healthChecker
	}
}

func (v *VM[I, O, A]) HealthCheck(ctx context.Context) (interface{}, error) {
	return v.Options.HealthChecker.HealthCheck(ctx)
}

func (o *Options[I, O, A]) WithHandler(name string, handler http.Handler) {
	o.Handlers[name] = handler
}

func WithHandler[I Block, O Block, A Block](name string, handler http.Handler) Option[I, O, A] {
	return func(opts *Options[I, O, A]) {
		opts.Handlers[name] = handler
	}
}

func (v *VM[I, O, A]) CreateHandlers(ctx context.Context) (map[string]http.Handler, error) {
	return v.Options.Handlers, nil
}

func (v *VM[I, O, A]) GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlockIDAtHeight")
	defer span.End()

	if blkHeight == v.lastAcceptedBlock.Height() {
		return v.lastAcceptedBlock.ID(), nil
	}
	if blkID, ok := v.acceptedBlocksByHeight.Get(blkHeight); ok {
		return blkID, nil
	}
	return v.chain.GetBlockIDAtHeight(ctx, blkHeight)
}

func (o *Options[I, O, A]) WithCloser(closer func() error) Option[I, O, A] {
	return func(opts *Options[I, O, A]) {
		opts.Closers = append(opts.Closers, closer)
	}
}

func (v *VM[I, O, A]) Shutdown(context.Context) error {
	close(v.shutdownChan)

	errs := make([]error, len(v.Options.Closers))
	for i, closer := range v.Options.Closers {
		errs[i] = closer()
	}
	return errors.Join(errs...)
}

func WithVersion[I Block, O Block, A Block](version string) Option[I, O, A] {
	return func(opts *Options[I, O, A]) {
		opts.Version = version
	}
}

func (v *VM[I, O, A]) Version(context.Context) (string, error) {
	return v.Options.Version, nil
}
