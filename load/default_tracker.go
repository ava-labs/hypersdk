// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"sync/atomic"
	"time"
)

var _ Tracker[any] = (*DefaultTracker[any])(nil)

type DefaultTracker[T comparable] struct {
	txIssuedCounter    atomic.Uint64
	txConfirmedCounter atomic.Uint64
	txFailedCounter    atomic.Uint64
}

func (t *DefaultTracker[T]) Issue(T, time.Time) {
	t.txIssuedCounter.Add(1)
}

func (t *DefaultTracker[T]) ObserveConfirmed(T, time.Time) {
	t.txConfirmedCounter.Add(1)
}

func (t *DefaultTracker[T]) ObserveFailed(T, time.Time) {
	t.txFailedCounter.Add(1)
}

func (t *DefaultTracker[T]) GetObservedConfirmed() uint64 {
	return t.txConfirmedCounter.Load()
}

func (t *DefaultTracker[T]) GetObservedFailed() uint64 {
	return t.txFailedCounter.Load()
}

func (t *DefaultTracker[T]) GetObservedIssued() uint64 {
	return t.txIssuedCounter.Load()
}
