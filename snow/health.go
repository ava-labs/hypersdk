// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/api/health"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
)

const (
	vmReadinessHealthChecker      = "snowVMReady"
	unresolvedBlocksHealthChecker = "snowUnresolvedBlocks"
)

var (
	errUnresolvedBlocks = errors.New("unresolved invalid blocks in processing")
	errVMNotReady       = errors.New("vm not ready")
)

// HealthCheck is a concrete implementation of health.Checker interface
// that reports the health and readiness of the VM.
//
// The health and readiness of the VM is determined by the health checkers registered with the VM.
//
// The health checkers are registered with the VM using RegisterHealthChecker.
func (v *VM[I, O, A]) HealthCheck(ctx context.Context) (any, error) {
	var (
		details = make(map[string]any)
		errs    []error
	)

	v.healthCheckers.Range(func(k, v any) bool {
		name := k.(string)
		checker := v.(health.Checker)
		checkerDetails, err := checker.HealthCheck(ctx)

		details[name] = checkerDetails
		errs = append(errs, err)
		return true
	})

	return details, errors.Join(errs...)
}

// RegisterHealthChecker registers a health checker with the VM
func (v *VM[I, O, A]) RegisterHealthChecker(name string, healthChecker health.Checker) error {
	if _, ok := v.healthCheckers.LoadOrStore(name, healthChecker); ok {
		return fmt.Errorf("duplicate health checker for %s", name)
	}

	return nil
}

func (v *VM[I, O, A]) initHealthCheckers() error {
	vmReadiness := newVMReadinessHealthCheck(func() bool {
		return v.ready
	})
	return v.RegisterHealthChecker(vmReadinessHealthChecker, vmReadiness)
}

// vmReadinessHealthCheck marks itself as ready iff the VM is in normal operation.
// ie. has the full state required to process new blocks from tip.
type vmReadinessHealthCheck struct {
	isReady func() bool
}

func newVMReadinessHealthCheck(isReady func() bool) *vmReadinessHealthCheck {
	return &vmReadinessHealthCheck{isReady: isReady}
}

func (v *vmReadinessHealthCheck) HealthCheck(_ context.Context) (any, error) {
	ready := v.isReady()
	if !ready {
		return ready, errVMNotReady
	}
	return ready, nil
}

// unresolvedBlockHealthCheck monitors blocks that require explicit resolution through the consensus process.
//
// During state sync, blocks are vacuously marked as verified because the VM lacks the state required
// to properly verify them. When state sync completes, these blocks must go through proper verification,
// but some may fail.
//
// This health check ensures chain integrity by tracking these blocks until they're explicitly
// rejected by consensus. Without such tracking:
//   - The node wouldn't differentiate between properly handled rejections and processing errors
//   - Chain state could become inconsistent if invalid blocks disappeared without formal rejection
//
// The health check reports unhealthy as long as any blocks remain unresolved, ensuring that
// the chain doesn't report full health until all blocks have been properly processed.
type unresolvedBlockHealthCheck[I Block] struct {
	lock             sync.RWMutex
	unresolvedBlocks set.Set[ids.ID]
}

func newUnresolvedBlocksHealthCheck[I Block](unresolvedBlkIDs set.Set[ids.ID]) *unresolvedBlockHealthCheck[I] {
	return &unresolvedBlockHealthCheck[I]{
		unresolvedBlocks: unresolvedBlkIDs,
	}
}

// Resolve marks a block as properly handled by the consensus process.
// This is called when a block is explicitly rejected, removing it from the unresolved set.
func (u *unresolvedBlockHealthCheck[I]) Resolve(blkID ids.ID) {
	u.lock.Lock()
	defer u.lock.Unlock()

	u.unresolvedBlocks.Remove(blkID)
}

// HealthCheck reports error if any blocks remain unresolved.
// Returns the count of unresolved blocks.
func (u *unresolvedBlockHealthCheck[I]) HealthCheck(_ context.Context) (any, error) {
	u.lock.RLock()
	unresolvedBlocks := u.unresolvedBlocks.Len()
	u.lock.RUnlock()

	if unresolvedBlocks > 0 {
		return unresolvedBlocks, fmt.Errorf("%w: %d", errUnresolvedBlocks, unresolvedBlocks)
	}
	return unresolvedBlocks, nil
}
