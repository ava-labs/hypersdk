// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"
	"sync"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	syncEng "github.com/ava-labs/avalanchego/x/sync"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/chain"
)

type stateSyncerClient struct {
	vm          *VM
	gatherer    ametrics.MultiGatherer
	syncManager *syncEng.StateSyncManager

	// tracks the sync target so we can update last accepted
	// block when sync completes.
	target        *chain.StatelessBlock
	targetUpdated bool

	// State Sync results
	init         bool
	startedSync  bool
	stateSyncErr error
	doneOnce     sync.Once
	done         chan struct{}
}

// TODO: break out into own package
func (vm *VM) NewStateSyncClient(
	gatherer ametrics.MultiGatherer,
) *stateSyncerClient {
	return &stateSyncerClient{
		vm:       vm,
		gatherer: gatherer,
		done:     make(chan struct{}),
	}
}

func (*stateSyncerClient) StateSyncEnabled(context.Context) (bool, error) {
	// We always start the state syncer and may fallback to normal bootstrapping
	// if we are close to tip.
	//
	// There is no way to trigger a full bootstrap from genesis.
	return true, nil
}

func (*stateSyncerClient) GetOngoingSyncStateSummary(
	context.Context,
) (block.StateSummary, error) {
	// Because the history of MerkleDB change proofs tends to be short, we always
	// restart syncing from scratch.
	//
	// This is unlike other DB implementations where roots are persisted
	// indefinitely (and it means we can continue from where we left off).
	return nil, database.ErrNotFound
}

func (s *stateSyncerClient) AcceptedSyncableBlock(
	_ context.Context,
	sb *chain.SyncableBlock,
) (block.StateSyncMode, error) {
	s.init = true
	// If we did not finish syncing, we must state sync.
	syncing, err := s.vm.GetDiskIsSyncing()
	if err != nil {
		s.vm.snowCtx.Log.Warn("could not determine if syncing", zap.Error(err))
		return block.StateSyncSkipped, err
	}
	if !syncing && (s.vm.lastAccepted.Hght+s.vm.config.GetStateSyncMinBlocks() > sb.Height()) {
		s.vm.snowCtx.Log.Info(
			"bypassing state sync",
			zap.Uint64("lastAccepted", s.vm.lastAccepted.Hght),
			zap.Uint64("syncableHeight", sb.Height()),
		)

		// We should backfill the emap if we are starting from the last accepted
		// block to avoid unnecessarily waiting for txs if we have recent blocks.
		s.startingSync(false)

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
	s.target = sb.StatelessBlock
	s.vm.snowCtx.Log.Info(
		"starting state sync",
		zap.Uint64("height", s.target.Hght),
		zap.Stringer("summary", sb),
		zap.Bool("already syncing", syncing),
	)

	// We don't backfill emap with old data because we are going to skip ahead
	// from the last accepted block.
	s.startingSync(true)

	// Initialize metrics for sync client
	r := prometheus.NewRegistry()
	metrics, err := syncEng.NewMetrics("sync_client", r)
	if err != nil {
		return block.StateSyncSkipped, err
	}
	if err := s.gatherer.Register("syncer", r); err != nil {
		return block.StateSyncSkipped, err
	}
	s.syncManager, err = syncEng.NewStateSyncManager(syncEng.StateSyncConfig{
		SyncDB: s.vm.stateDB,
		Client: syncEng.NewClient(&syncEng.ClientConfig{
			NetworkClient:    s.vm.stateSyncNetworkClient,
			Log:              s.vm.snowCtx.Log,
			Metrics:          metrics,
			StateSyncNodeIDs: nil, // pull from all
		}),
		SimultaneousWorkLimit: s.vm.config.GetStateSyncParallelism(),
		Log:                   s.vm.snowCtx.Log,
		TargetRoot:            sb.StateRoot,
	})
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
	if err := s.target.SetLastAccepted(context.Background()); err != nil {
		return block.StateSyncSkipped, err
	}

	// Kickoff state syncing from [s.target]
	if err := s.syncManager.StartSyncing(context.Background()); err != nil {
		s.vm.snowCtx.Log.Warn("not starting state syncing", zap.Error(err))
		return block.StateSyncSkipped, err
	}
	go func() {
		// wait for the work to complete on this goroutine
		//
		// [syncManager] guarantees this will always return so it isn't possible to
		// deadlock.
		s.stateSyncErr = s.syncManager.Wait(context.Background())
		s.vm.snowCtx.Log.Info("state sync done", zap.Error(s.stateSyncErr))
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
func (s *stateSyncerClient) finishSync() error {
	if s.targetUpdated {
		// Will look like block on start accepted then last block before beginning
		// bootstrapping is accepted.
		//
		// NOTE: There may be a number of verified but unaccepted blocks above this
		// block.
		if err := s.target.SetLastAccepted(context.Background()); err != nil {
			return err
		}
	}
	return s.vm.PutDiskIsSyncing(false)
}

func (s *stateSyncerClient) Started() bool {
	return s.startedSync
}

// ForceDone is used by the [VM] to skip the sync process or to close the
// channel if the sync process never started (i.e. [AcceptedSyncableBlock] will
// never be called)
func (s *stateSyncerClient) ForceDone() {
	if s.startedSync {
		// If we started sync, we must wait for it to finish
		return
	}
	s.doneOnce.Do(func() {
		close(s.done)
	})
}

// Shutdown can be called to abort an ongoing sync.
func (s *stateSyncerClient) Shutdown() error {
	if s.syncManager != nil {
		s.syncManager.Close()
		<-s.done // wait for goroutine to exit
	}
	return s.stateSyncErr // will be nil if [syncManager] is nil
}

// Error returns a non-nil error if one occurred during the sync.
func (s *stateSyncerClient) Error() error { return s.stateSyncErr }

func (s *stateSyncerClient) StateReady() bool {
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
func (s *stateSyncerClient) UpdateSyncTarget(b *chain.StatelessBlock) (bool, error) {
	err := s.syncManager.UpdateSyncTarget(b.StateRoot)
	if errors.Is(err, syncEng.ErrAlreadyClosed) {
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

// startingSync is called before [AcceptedSyncableBlock] returns
func (s *stateSyncerClient) startingSync(state bool) {
	s.startedSync = true
	vm := s.vm

	// If state sync, we pessimistically assume nothing we have on-disk will
	// be useful (as we will jump ahead to some future block).
	if state {
		return
	}

	// Exit early if we don't have any blocks other than genesis (which
	// contains no transactions)
	blk := vm.lastAccepted
	if blk.Hght == 0 {
		vm.snowCtx.Log.Info("no seen transactions to backfill")
		vm.startSeenTime = 0
		vm.seenValidityWindowOnce.Do(func() {
			close(vm.seenValidityWindow)
		})
		return
	}

	// Backfill [vm.seen] with lifeline worth of transactions
	r := vm.Rules(vm.lastAccepted.Tmstmp)
	oldest := uint64(0)
	var err error
	for {
		if vm.lastAccepted.Tmstmp-blk.Tmstmp > r.GetValidityWindow() {
			// We are assured this function won't be running while we accept
			// a block, so we don't need to protect against closing this channel
			// twice.
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// It is ok to add transactions from newest to oldest
		vm.seen.Add(blk.Txs)
		vm.startSeenTime = blk.Tmstmp
		oldest = blk.Hght

		// Exit early if next block to fetch is genesis (which contains no
		// txs)
		if blk.Hght <= 1 {
			// If we have walked back from the last accepted block to genesis, then
			// we can be sure we have all required transactions to start validation.
			vm.startSeenTime = 0
			vm.seenValidityWindowOnce.Do(func() {
				close(vm.seenValidityWindow)
			})
			break
		}

		// Set next blk in lookback
		blk, err = vm.GetStatelessBlock(context.Background(), blk.Prnt)
		if err != nil {
			vm.snowCtx.Log.Error("could not load block, exiting backfill", zap.Error(err))
			break
		}
	}
	vm.snowCtx.Log.Info(
		"backfilled seen txs",
		zap.Uint64("start", oldest),
		zap.Uint64("finish", vm.lastAccepted.Hght),
	)
}
