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

// stateSyncHealthChecker is a concrete implementation of a health check that monitors the state of the VM
// during and after dynamic state synchronization. It ensures the following:
//  1. The VM is ready for normal operation (`vm.ready` is true).
//  2. Tracks `unresolvedBlocks`, which are outstanding processing blocks that were vacuously verified.
//     These blocks were marked as successfully verified even though the VM lacked the necessary state
//     information to properly verify them at the time.
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

	vm.AddRejectedSub(event.SubscriptionFunc[O]{
		NotifyF: func(_ context.Context, output O) error {
			s.lock.Lock()
			defer s.lock.Unlock()

			fmt.Printf("Received: %s\n", output.GetID())
			delete(s.unresolvedBlocks, output.GetID())
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

func (s *stateSyncHealthChecker[I, O, A]) MarkFailed(blkID ids.ID) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.unresolvedBlocks[blkID] = struct{}{}
}
