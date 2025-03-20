// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"sync"
	"time"
)

var _ Tracker[any] = (*DefaultTracker[any])(nil)

type DefaultTracker[T comparable] struct {
	sync.RWMutex

	txIssuedCounter    uint64
	txConfirmedCounter uint64
	txFailedCounter    uint64
}

func (t *DefaultTracker[T]) Issue(T, time.Time) {
	t.Lock()
	t.txIssuedCounter++
	t.Unlock()
}

func (t *DefaultTracker[T]) ObserveConfirmed(T) {
	t.Lock()
	defer t.Unlock()
	t.txConfirmedCounter++
}

func (t *DefaultTracker[T]) ObserveFailed(T) {
	t.Lock()
	defer t.Unlock()
	t.txFailedCounter++
}

func (t *DefaultTracker[T]) GetObservedConfirmed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txConfirmedCounter
}

func (t *DefaultTracker[T]) GetObservedFailed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txFailedCounter
}

func (t *DefaultTracker[T]) GetObservedIssued() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txIssuedCounter
}
