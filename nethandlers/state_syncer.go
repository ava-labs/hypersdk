// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package nethandlers

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/x/merkledb"
	avasync "github.com/ava-labs/avalanchego/x/sync"
)

type StateSyncerClientVM interface {
	Logger() logging.Logger
	GetDiskIsSyncing() (bool, error)
	LastAcceptedBlock() *chain.StatefulBlock
	GetStateSyncMinBlocks() uint64
	GetStateSyncParallelism() int
	Genesis() genesis.Genesis
	State() (merkledb.MerkleDB, error)
	Network() *p2p.Network
	PutDiskIsSyncing(v bool) error
}

type StateSyncerClient struct {
	vm          StateSyncerClientVM
	gatherer    avametrics.MultiGatherer
	syncManager *avasync.Manager

	// tracks the sync target so we can update last accepted
	// block when sync completes.
	target        *chain.StatefulBlock
	targetUpdated bool

	// State Sync results
	init         bool
	startedSync  bool
	stateSyncErr error
	doneOnce     sync.Once
	done         chan struct{}
}

func NewStateSyncClient(
	gatherer avametrics.MultiGatherer,
	vm StateSyncerClientVM,
) *StateSyncerClient {
	return &StateSyncerClient{
		vm:       vm,
		gatherer: gatherer,
		done:     make(chan struct{}),
	}
}

func (*StateSyncerClient) StateSyncEnabled(context.Context) (bool, error) {
	// We always start the state syncer and may fallback to normal bootstrapping
	// if we are close to tip.
	//
	// There is no way to trigger a full bootstrap from genesis.
	return true, nil
}

func (*StateSyncerClient) GetOngoingSyncStateSummary(
	context.Context,
) (block.StateSummary, error) {
	// Because the history of MerkleDB change proofs tends to be short, we always
	// restart syncing from scratch.
	//
	// This is unlike other DB implementations where roots are persisted
	// indefinitely (and it means we can continue from where we left off).
	return nil, database.ErrNotFound
}

func (s *StateSyncerClient) AcceptedSyncableBlock(
	_ context.Context,
	sb *chain.SyncableBlock,
) (block.StateSyncMode, error) {
	s.init = true
	s.vm.Logger().Info("accepted syncable block",
		zap.Uint64("height", sb.Height()),
		zap.Stringer("blockID", sb.ID()),
	)

	// If we did not finish syncing, we must state sync.
	syncing, err := s.vm.GetDiskIsSyncing()
	if err != nil {
		s.vm.Logger().Warn("could not determine if syncing", zap.Error(err))
		return block.StateSyncSkipped, err
	}
	if !syncing && (s.vm.LastAcceptedBlock().Hght+s.vm.GetStateSyncMinBlocks() > sb.Height()) {
		s.vm.Logger().Info(
			"bypassing state sync",
			zap.Uint64("lastAccepted", s.vm.LastAcceptedBlock().Hght),
			zap.Uint64("syncableHeight", sb.Height()),
		)
		s.startedSync = true

		// We trigger [done] immediately so we let the engine know we are
		// synced as soon as the [ValidityWindow] worth of txs are verified.
		s.doneOnce.Do(func() {
			close(s.done)
		})

		// Even when we do normal bootstrapping, we mark syncing as dynamic to
		// ensure we fill [vm.seen] before transitioning to normal operation.
		//
		// If there is no last accepted block above genesis, we will perform normal
		// bootstrapping before transitioning into normal operation.
		return block.StateSyncDynamic, nil
	}

	// When state syncing after restart (whether successful or not), we restart
	// from scratch.
	//
	// MerkleDB will handle clearing any keys on-disk that are no
	// longer necessary.
	s.target = sb.StatefulBlock
	s.vm.Logger().Info(
		"starting state sync",
		zap.Uint64("height", s.target.Hght),
		zap.Stringer("summary", sb),
		zap.Bool("already syncing", syncing),
	)
	s.startedSync = true

	// Initialize metrics for sync client
	r := prometheus.NewRegistry()
	if err := s.gatherer.Register("syncer", r); err != nil {
		return block.StateSyncSkipped, err
	}
	stateDB, err := s.vm.State()
	if err != nil {
		return block.StateSyncSkipped, err
	}
	s.syncManager, err = avasync.NewManager(avasync.ManagerConfig{
		BranchFactor:          s.vm.Genesis().GetStateBranchFactor(),
		DB:                    stateDB,
		RangeProofClient:      s.vm.Network().NewClient(RangeProofHandlerID),
		ChangeProofClient:     s.vm.Network().NewClient(ChangeProofHandlerID),
		SimultaneousWorkLimit: s.vm.GetStateSyncParallelism(),
		Log:                   s.vm.Logger(),
		TargetRoot:            sb.StateRoot,
	}, r)
	if err != nil {
		return block.StateSyncSkipped, err
	}

	// Persist that the node has started syncing.
	//
	// This is necessary since last accepted will be modified without
	// the VM having state, so it must resume only in state-sync
	// mode if interrupted.
	//
	// Since the sync will write directly into the state trie,
	// the node cannot continue from the previous state once
	// it starts state syncing.
	if err := s.vm.PutDiskIsSyncing(true); err != nil {
		return block.StateSyncSkipped, err
	}

	// Update the last accepted to the state target block,
	// since we don't want bootstrapping to fetch all the blocks
	// from genesis to the sync target.
	s.target.MarkAccepted(context.Background())

	// Kickoff state syncing from [s.target]
	if err := s.syncManager.Start(context.Background()); err != nil {
		s.vm.Logger().Warn("not starting state syncing", zap.Error(err))
		return block.StateSyncSkipped, err
	}
	go func() {
		// wait for the work to complete on this goroutine
		//
		// [syncManager] guarantees this will always return so it isn't possible to
		// deadlock.
		s.stateSyncErr = s.syncManager.Wait(context.Background())
		s.vm.Logger().Info("state sync done", zap.Error(s.stateSyncErr))
		if s.stateSyncErr == nil {
			// if the sync was successful, update the last accepted pointers.
			s.stateSyncErr = s.finishSync()
		}
		// notify the engine the VM is ready to participate
		// in voting and it can verify blocks.
		//
		// This function will send a message to the VM when it has processed at least
		// [ValidityWindow] blocks.
		s.doneOnce.Do(func() {
			close(s.done)
		})
	}()
	// TODO: engine will mark VM as ready when we return
	// [block.StateSyncDynamic]. This should change in v1.9.11.
	return block.StateSyncDynamic, nil
}

// finishSync is responsible for updating disk and memory pointers
func (s *StateSyncerClient) finishSync() error {
	if s.targetUpdated {
		// Will look like block on start accepted then last block before beginning
		// bootstrapping is accepted.
		//
		// NOTE: There may be a number of verified but unaccepted blocks above this
		// block.
		s.target.MarkAccepted(context.Background())
	}
	return s.vm.PutDiskIsSyncing(false)
}

func (s *StateSyncerClient) Started() bool {
	return s.startedSync
}

// ForceDone is used by the [VM] to skip the sync process or to close the
// channel if the sync process never started (i.e. [AcceptedSyncableBlock] will
// never be called)
func (s *StateSyncerClient) ForceDone() {
	if s.startedSync {
		// If we started sync, we must wait for it to finish
		return
	}
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

// Shutdown can be called to abort an ongoing sync.
func (s *StateSyncerClient) Shutdown() error {
	if s.syncManager != nil {
		s.syncManager.Close()
		<-s.done // wait for goroutine to exit
	}
	return s.stateSyncErr // will be nil if [syncManager] is nil
}

// Error returns a non-nil error if one occurred during the sync.
func (s *StateSyncerClient) Error() error { return s.stateSyncErr }

func (s *StateSyncerClient) StateReady() bool {
	select {
	case <-s.done:
		return true
	default:
	}
	// If we have not yet invoked [AcceptedSyncableBlock] we should return
	// false until it has been called or we invoke [ForceDone].
	if !s.init {
		return false
	}
	// Cover the case where initialization failed
	return s.syncManager == nil
}

// UpdateSyncTarget returns a boolean indicating if the root was
// updated and an error if one occurred while updating the root.
func (s *StateSyncerClient) UpdateSyncTarget(b *chain.StatefulBlock) (bool, error) {
	err := s.syncManager.UpdateSyncTarget(b.StateRoot)
	if errors.Is(err, avasync.ErrAlreadyClosed) {
		<-s.done          // Wait for goroutine to exit for consistent return values with IsSyncing
		return false, nil // Sync finished before update
	}
	if err != nil {
		return false, err // Unexpected error
	}
	s.target = b           // Remember the new target
	s.targetUpdated = true // Set [targetUpdated] so we call SetLastAccepted on finish
	return true, nil       // Sync root target updated successfully
}

func (s *StateSyncerClient) Done() <-chan struct{} {
	return s.done
}
