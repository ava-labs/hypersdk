// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/event"
	"sync"
)

const StateSyncHealthCheckerNamespace = "stateSyncHealthChecker"

var (
	ErrVMNotReady       = errors.New("vm is not ready")
	ErrUnresolvedBlocks = errors.New("blocks remain unresolved after verification and must be explicitly rejected")
)

// stateSyncHealthChecker monitors VM health during and after dynamic state sync.
// During state sync, blocks are vacuously marked as verified because the VM is missing the current state.
// Consensus will eventually reject any invalid blocks, but this check ensures the VM waits to report healthy until it has cleared all invalid blocks from the processing set.
//
// The health checker ensures:
//  1. VM readiness; Tracks if VM is ready for normal operation (`vm.ready`)
//  2. Block resolution; Monitors blocks that were vacuously verified during state sync
//     and reports unhealthy status if any remain unresolved
//
// Safety guarantee: We rely on consensus and correct validator set that any invalid blocks
// accepted during state sync will eventually be rejected in favor of valid blocks.
type stateSyncHealthChecker[I, O, A Block] struct {
	vm               *VM[I, O, A]
	unresolvedBlocks map[ids.ID]struct{}
	lock             sync.RWMutex
}

func newStateSyncHealthChecker[I, O, A Block](vm *VM[I, O, A]) *stateSyncHealthChecker[I, O, A] {
	s := &stateSyncHealthChecker[I, O, A]{
		vm:               vm,
		unresolvedBlocks: make(map[ids.ID]struct{}),
	}

	vm.AddPreRejectedSub(event.SubscriptionFunc[I]{
		NotifyF: func(_ context.Context, input I) error {
			s.lock.Lock()
			defer s.lock.Unlock()

			delete(s.unresolvedBlocks, input.GetID())
			return nil
		},
	})

	return s
}

func (s *stateSyncHealthChecker[I, O, A]) HealthCheck(_ context.Context) (interface{}, error) {
	details := map[string]interface{}{
		"ready": s.vm.ready,
	}

	// Report unhealthy while VM is not ready
	if !s.vm.ready {
		return details, ErrVMNotReady
	}

	// Check vacuously verified blocks
	s.lock.RLock()
	unresolvedNum := len(s.unresolvedBlocks)
	s.lock.RUnlock()

	details["unresolvedBlocks"] = unresolvedNum
	if unresolvedNum > 0 {
		return details, fmt.Errorf("%w: %d block(s)\n", ErrUnresolvedBlocks, unresolvedNum)
	}

	return details, nil
}

func (s *stateSyncHealthChecker[I, O, A]) MarkUnresolved(blkID ids.ID) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.unresolvedBlocks[blkID] = struct{}{}
}
