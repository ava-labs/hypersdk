// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/event"
	"sync"
)

const (
	VMReadinessHealthChecker      = "snowVMReadiness"
	UnresolvedBlocksHealthChecker = "snowUnresolvedBlocks"
)

func (v *VM[I, O, A]) RegisterHealthChecker(name string, healthChecker health.Checker) error {
	if _, loaded := v.healthCheckers.LoadOrStore(name, healthChecker); loaded {
		return fmt.Errorf("duplicate health checker for %s", name)
	}

	return nil
}

func (v *VM[I, O, A]) HealthChecker(name string) (interface{}, error) {
	value, ok := v.healthCheckers.Load(name)
	if !ok {
		return nil, fmt.Errorf("health checker %s not found", name)
	}

	return value, nil
}

func (v *VM[I, O, A]) initHealthCheckers() error {
	vmReadiness := NewVMReadiness(func() bool {
		return v.ready
	})
	if err := v.RegisterHealthChecker(VMReadinessHealthChecker, vmReadiness); err != nil {
		return err
	}

	unresolvedBlockHealthChecker := NewUnresolvedBlocksCheck[I]()
	if err := v.RegisterHealthChecker(UnresolvedBlocksHealthChecker, unresolvedBlockHealthChecker); err != nil {
		return err
	}

	// Remove vacuously verified blocks that has been rejected
	// Passing health check
	v.AddPreRejectedSub(unresolvedBlockHealthChecker.Resolve())

	return nil
}

// VMReadiness is concrete health check that should mark itself ready if isReady function returns true
type VMReadiness struct {
	isReady func() bool
}

func NewVMReadiness(isReady func() bool) *VMReadiness {
	return &VMReadiness{isReady: isReady}
}

func (v *VMReadiness) HealthCheck(_ context.Context) (interface{}, error) {
	ready := v.isReady()
	if !ready {
		return nil, fmt.Errorf("vm is not ready")
	}

	details := map[string]interface{}{
		"ready": ready,
	}
	return details, nil
}

func (v *VMReadiness) Name() string {
	return "snowVMReadiness"
}

// UnresolvedBlocksCheck
// During state sync, blocks are vacuously marked as verified because the VM is missing the current state.
// Consensus will eventually reject any invalid blocks, but this check ensures the VM waits to report healthy until it has cleared all invalid blocks from the processing set.
// The health checker monitors blocks that were vacuously verified during state sync and reports unhealthy status if any remain unresolved
//
// Safety guarantee: We rely on consensus and correct validator set that any invalid blocks
// accepted during state sync will eventually be rejected in favor of valid blocks.
type UnresolvedBlocksCheck[I Block] struct {
	unresolvedBlocks set.Set[ids.ID]
	lock             sync.RWMutex
}

func NewUnresolvedBlocksCheck[I Block]() *UnresolvedBlocksCheck[I] {
	return &UnresolvedBlocksCheck[I]{
		unresolvedBlocks: set.Set[ids.ID]{},
	}
}

func (u *UnresolvedBlocksCheck[I]) Resolve() event.SubscriptionFunc[I] {
	return event.SubscriptionFunc[I]{
		NotifyF: func(ctx context.Context, input I) error {
			u.lock.Lock()
			defer u.lock.Unlock()

			u.unresolvedBlocks.Remove(input.GetID())
			return nil
		},
	}
}

func (u *UnresolvedBlocksCheck[I]) MarkUnresolved(unresolvedBlkID ids.ID) {
	u.lock.Lock()
	defer u.lock.Unlock()

	u.unresolvedBlocks.Add(unresolvedBlkID)
}

func (u *UnresolvedBlocksCheck[I]) HealthCheck(_ context.Context) (interface{}, error) {
	u.lock.RLock()
	unresolvedBlocks := u.unresolvedBlocks.Len()
	u.lock.RUnlock()

	details := map[string]interface{}{
		"unresolvedBlocks": unresolvedBlocks,
	}
	if unresolvedBlocks > 0 {
		return details, fmt.Errorf("blocks remain unresolved after verification and must be explicitly rejected: %d blocks", unresolvedBlocks)
	}
	return details, nil
}
