// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

var _ Tracker = (*DefaultTracker)(nil)

type DefaultTracker struct {
	sync.RWMutex

	txIssuedCounter    uint64
	txConfirmedCounter uint64
	txFailedCounter    uint64
}

func (t *DefaultTracker) Issue(ids.ID, time.Time) {
	t.Lock()
	t.txIssuedCounter++
	t.Unlock()
}

func (t *DefaultTracker) ObserveConfirmed(ids.ID) {
	t.Lock()
	defer t.Unlock()
	t.txConfirmedCounter++
}

func (t *DefaultTracker) ObserveFailed(ids.ID) {
	t.Lock()
	defer t.Unlock()
	t.txFailedCounter++
}

func (t *DefaultTracker) GetObservedConfirmed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txConfirmedCounter
}

func (t *DefaultTracker) GetObservedFailed() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txFailedCounter
}

func (t *DefaultTracker) GetObservedIssued() uint64 {
	t.RLock()
	defer t.RUnlock()
	return t.txIssuedCounter
}
