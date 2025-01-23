package snow

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/event"
	"sync"
)

type stateSyncHealthChecker[I, O, A Block] struct {
	vm           *VM[I, O, A]
	failedBlocks map[ids.ID]struct{}
	lock         sync.RWMutex
}

func newStateSyncHealthChecker[I, O, A Block](vm *VM[I, O, A]) *stateSyncHealthChecker[I, O, A] {
	s := &stateSyncHealthChecker[I, O, A]{
		vm:           vm,
		failedBlocks: make(map[ids.ID]struct{}),
	}

	vm.AddRejectedSub(event.SubscriptionFunc[O]{
		NotifyF: func(_ context.Context, output O) error {
			s.lock.Lock()
			defer s.lock.Unlock()

			delete(s.failedBlocks, output.GetID())
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
		return details, fmt.Errorf("vm is not ready")
	}

	// Check vacuously verified blocks
	s.lock.RLock()
	numFailed := len(s.failedBlocks)
	s.lock.RUnlock()

	details["failedBlocks"] = numFailed
	if numFailed > 0 {
		return details, fmt.Errorf("%d blocks failed verification and must be rejected", numFailed)
	}

	return details, nil
}

func (s *stateSyncHealthChecker[I, O, A]) MarkFailed(blkID ids.ID) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.failedBlocks[blkID] = struct{}{}
}
