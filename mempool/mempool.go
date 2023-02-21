// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
)

type Mempool struct {
	tracer trace.Tracer

	mu sync.RWMutex

	maxSize      int
	maxPayerSize int // Maximum txs allowed by a single payer

	pm *SortedMempool // Price Mempool
	tm *SortedMempool // Time Mempool

	// [Owned] used to remove all txs from an account when the balance is
	// insufficient
	owned map[string]map[ids.ID]struct{}

	// payers that are exempt from [maxPayerSize]
	exemptPayers map[string]struct{}
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New(tracer trace.Tracer, maxSize int, maxPayerSize int, exemptPayers [][]byte) *Mempool {
	m := &Mempool{
		tracer: tracer,

		maxSize:      maxSize,
		maxPayerSize: maxPayerSize,

		pm: NewSortedMempool(
			maxSize, /* pre-allocate total size */
			func(tx Item) uint64 { return tx.UnitPrice() },
		),
		tm: NewSortedMempool(
			maxSize, /* pre-allocate total size */
			func(tx Item) uint64 { return uint64(tx.Expiry()) },
		),
		owned:        map[string]map[ids.ID]struct{}{},
		exemptPayers: map[string]struct{}{},
	}
	for _, payer := range exemptPayers {
		m.exemptPayers[string(payer)] = struct{}{}
	}
	return m
}

func (th *Mempool) removeFromOwned(tx Item) {
	sender := tx.Payer()
	acct, ok := th.owned[sender]
	if !ok {
		// May no longer be populated
		return
	}
	delete(acct, tx.ID())
	if len(acct) == 0 {
		delete(th.owned, sender)
	}
}

// Has returns if the pm of [th] contains [txID]
func (th *Mempool) Has(ctx context.Context, txID ids.ID) bool {
	_, span := th.tracer.Start(ctx, "Mempool.Has")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()
	return th.pm.Has(txID)
}

// Add pushes all new txs from [txs] to th. Does not add a tx if
// the tx payer is not exempt and their txs in the mempool exceed th.maxPayerSize.
// If the size of th exceeds th.maxSize, Add pops the lowest value item
// from th.pm.
func (th *Mempool) Add(ctx context.Context, txs []Item) {
	_, span := th.tracer.Start(ctx, "Mempool.Add")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, tx := range txs {
		sender := tx.Payer()

		// Ensure no duplicate
		if th.pm.Has(tx.ID()) {
			// Don't drop because already exists
			continue
		}

		// Optimistically add to both mempools
		acct, ok := th.owned[sender]
		if !ok {
			acct = map[ids.ID]struct{}{}
			th.owned[sender] = acct
		}
		_, exempt := th.exemptPayers[sender]
		if !exempt && len(acct) == th.maxPayerSize {
			continue // do nothing, wait for txs to expire
		}
		th.pm.Add(tx)
		th.tm.Add(tx)
		acct[tx.ID()] = struct{}{}

		// Remove the lowest paying tx if at global max
		if th.pm.Len() > th.maxSize {
			// Remove the lowest paying tx
			lowTx := th.pm.PopMin()
			th.tm.Remove(lowTx.ID())
			th.removeFromOwned(lowTx)
		}
	}
}

// PeekMax returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool) PeekMax(ctx context.Context) Item {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMax")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMax()
}

// PeekMin returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool) PeekMin(ctx context.Context) Item {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMin")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMin()
}

// PopMax removes and returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool) PopMax(ctx context.Context) Item { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMax")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	max := th.pm.PopMax()
	th.tm.Remove(max.ID())
	th.removeFromOwned(max)
	return max
}

// PopMin removes and returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool) PopMin(ctx context.Context) Item { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMin")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	min := th.pm.PopMin()
	th.tm.Remove(min.ID())
	th.removeFromOwned(min)
	return min
}

// Remove removes [txs] from th.
func (th *Mempool) Remove(ctx context.Context, txs []Item) {
	_, span := th.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, tx := range txs {
		th.pm.Remove(tx.ID())
		th.tm.Remove(tx.ID())
		th.removeFromOwned(tx)
		// Remove is called when verifying a block. We should not drop transactions at
		// this time.
	}
}

// Len returns the number of txs in th.
func (th *Mempool) Len(ctx context.Context) int {
	_, span := th.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.Len()
}

// RemoveAccount removes all txs by [sender] from th.
func (th *Mempool) RemoveAccount(ctx context.Context, sender string) {
	_, span := th.tracer.Start(ctx, "Mempool.RemoveAccount")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.removeAccount(sender)
}

func (th *Mempool) removeAccount(sender string) {
	acct, ok := th.owned[sender]
	if !ok {
		return
	}
	for tx := range acct {
		th.pm.Remove(tx)
		th.tm.Remove(tx)
	}
	delete(th.owned, sender)
}

// SetMinTimestamp removes all txs with a lower expiry than [t] from th.
// SetMinTimestamp returns the list of removed txs.
func (th *Mempool) SetMinTimestamp(ctx context.Context, t int64) []Item {
	_, span := th.tracer.Start(ctx, "Mempool.SetMinTimesamp")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	removed := th.tm.SetMinVal(uint64(t))
	for _, remove := range removed {
		th.pm.Remove(remove.ID())
		th.removeFromOwned(remove)
	}
	return removed
}

func (th *Mempool) Build(
	ctx context.Context,
	f func(context.Context, Item) (cont bool, restore bool, removeAcct bool, err error),
) error {
	ctx, span := th.tracer.Start(ctx, "Mempool.Build")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	restorableTxs := []Item{}
	var err error
	for th.pm.Len() > 0 {
		max := th.pm.PopMax()
		cont, restore, removeAccount, fErr := f(ctx, max)
		if restore {
			// Waiting to restore unused transactions ensures that an account will be
			// excluded from future price mempool iterations
			restorableTxs = append(restorableTxs, max)
		} else {
			th.tm.Remove(max.ID())
			th.removeFromOwned(max)
		}
		if removeAccount {
			// We remove the account typically when the next execution results in an
			// invalid balance
			th.removeAccount(max.GetPayer())
		}
		if !cont || fErr != nil {
			err = fErr
			break
		}
	}
	//
	// Restore unused txs
	for _, tx := range restorableTxs {
		th.pm.Add(tx)
	}
	return err
}
