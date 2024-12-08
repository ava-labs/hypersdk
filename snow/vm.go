// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/version"
)

var (
	_ block.ChainVM         = (*VM[snowman.Block])(nil)
	_ block.StateSyncableVM = (*VM[snowman.Block])(nil)
)

type ChainInput struct {
	SnowCtx                                 *snow.Context
	GenesisBytes, UpgradeBytes, ConfigBytes []byte
	ToEngine                                chan<- common.Message
	AppSender                               common.AppSender
}

type ConcreteVM[T snowman.Block] interface {
	Initialize(
		ctx context.Context,
		chainInput ChainInput,
		options *Options,
	) error
	SetState(ctx context.Context, state snow.State) error
	Shutdown(context.Context) error
	Version(context.Context) (string, error)
	CreateHandlers(context.Context) (map[string]http.Handler, error)
	GetBlock(ctx context.Context, id ids.ID) (T, error)
	ParseBlock(ctx context.Context, source []byte) (T, error)
	BuildBlock(ctx context.Context) (T, error)
	SetPreference(ctx context.Context, blkID ids.ID) error
	LastAccepted(context.Context) (ids.ID, error)
	GetBlockIDAtHeight(ctx context.Context, height uint64) (ids.ID, error)
}

type VM[T snowman.Block] struct {
	ConcreteVM[T]
	Options Options
}

type Options struct {
	HealthChecker   health.Checker
	Connector       validators.Connector
	AppHandler      common.AppHandler
	StateSyncableVM block.StateSyncableVM
}



type Option func(*Options)

func WithHealthChecker(healthChecker health.Checker) Option {
	return func(opts *Options) {
		opts.HealthChecker = healthChecker
	}
}

func WithP2PNetwork(network *p2p.Network) Option {
	return func(opts *Options) {
		opts.Connector = network
		opts.AppHandler = network
	}
}

func WithConnector(connector validators.Connector) Option {
	return func(opts *Options) {
		opts.Connector = connector
	}
}

func WithAppHandler(appHandler common.AppHandler) Option {
	return func(opts *Options) {
		opts.AppHandler = appHandler
	}
}

func WithStateSyncableVM(stateSyncableVM block.StateSyncableVM) Option {
	return func(opts *Options) {
		opts.StateSyncableVM = stateSyncableVM
	}
}

func NewVM[T snowman.Block](concreteVM ConcreteVM[T]) *VM[T] {
	return &VM[T]{
		ConcreteVM: concreteVM,
		Options: Options{
			HealthChecker: health.CheckerFunc(func(ctx context.Context) (interface{}, error) {
				return nil, nil
			}),
			AppHandler: common.NewNoOpAppHandler(logging.NoLog{}),
		},
	}
}

func (v *VM[T]) Initialize(
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
	chainInput := ChainInput{
		SnowCtx:      chainCtx,
		GenesisBytes: genesisBytes,
		UpgradeBytes: upgradeBytes,
		ConfigBytes:  configBytes,
		ToEngine:     toEngine,
		AppSender:    appSender,
	}
	return v.ConcreteVM.Initialize(ctx, chainInput, &v.Options)
}

func (v *VM[T]) AppRequest(ctx context.Context, nodeID ids.NodeID, requestID uint32, deadline time.Time, request []byte) error {
	return v.Options.AppHandler.AppRequest(ctx, nodeID, requestID, deadline, request)
}

func (v *VM[T]) AppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
	return v.Options.AppHandler.AppResponse(ctx, nodeID, requestID, response)
}

func (v *VM[T]) AppRequestFailed(ctx context.Context, nodeID ids.NodeID, requestID uint32, appErr *common.AppError) error {
	return v.Options.AppHandler.AppRequestFailed(ctx, nodeID, requestID, appErr)
}

func (v *VM[T]) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	return v.Options.AppHandler.AppGossip(ctx, nodeID, msg)
}

func (v *VM[T]) HealthCheck(ctx context.Context) (interface{}, error) {
	return v.Options.HealthChecker.HealthCheck(ctx)
}

func (v *VM[T]) GetBlock(ctx context.Context, id ids.ID) (snowman.Block, error) {
	return v.ConcreteVM.GetBlock(ctx, id)
}

func (v *VM[T]) ParseBlock(ctx context.Context, source []byte) (snowman.Block, error) {
	return v.ConcreteVM.ParseBlock(ctx, source)
}

func (v *VM[T]) BuildBlock(ctx context.Context) (snowman.Block, error) {
	return v.ConcreteVM.BuildBlock(ctx)
}

func (v *VM[T]) Connected(ctx context.Context, nodeID ids.NodeID, nodeVersion *version.Application) error {
	return v.Options.Connector.Connected(ctx, nodeID, nodeVersion)
}

func (v *VM[T]) Disconnected(ctx context.Context, nodeID ids.NodeID) error {
	return v.Options.Connector.Disconnected(ctx, nodeID)
}

func (v *VM[T]) StateSyncEnabled(ctx context.Context) (bool, error) {
	return v.Options.StateSyncableVM.StateSyncEnabled(ctx)
}

func (v *VM[T]) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.Options.StateSyncableVM.GetOngoingSyncStateSummary(ctx)
}

func (v *VM[T]) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.Options.StateSyncableVM.GetLastStateSummary(ctx)
}

func (v *VM[T]) ParseStateSummary(ctx context.Context, summaryBytes []byte) (block.StateSummary, error) {
	return v.Options.StateSyncableVM.ParseStateSummary(ctx, summaryBytes)
}

func (v *VM[T]) GetStateSummary(ctx context.Context, summaryHeight uint64) (block.StateSummary, error) {
	return v.Options.StateSyncableVM.GetStateSummary(ctx, summaryHeight)
}
