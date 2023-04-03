// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
)

const maxPrealloc = 4_096

type Mempool[T Item] struct {
	tracer trace.Tracer

	mu sync.RWMutex

	maxSize      int
	maxPayerSize int // Maximum items allowed by a single payer

	pm *SortedMempool[T] // Price Mempool
	tm *SortedMempool[T] // Time Mempool

	// [Owned] used to remove all items from an account when the balance is
	// insufficient
	owned map[string]set.Set[ids.ID]

	// payers that are exempt from [maxPayerSize]
	exemptPayers set.Set[string]
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New[T Item](
	tracer trace.Tracer,
	maxSize int,
	maxPayerSize int,
	exemptPayers [][]byte,
) *Mempool[T] {
	m := &Mempool[T]{
		tracer: tracer,

		maxSize:      maxSize,
		maxPayerSize: maxPayerSize,

		pm: NewSortedMempool(
			math.Min(maxSize, maxPrealloc),
			func(item T) uint64 { return item.UnitPrice() },
		),
		tm: NewSortedMempool(
			math.Min(maxSize, maxPrealloc),
			func(item T) uint64 { return uint64(item.Expiry()) },
		),
		owned:        map[string]set.Set[ids.ID]{},
		exemptPayers: set.Set[string]{},
	}
	for _, payer := range exemptPayers {
		m.exemptPayers.Add(string(payer))
	}
	return m
}

func (th *Mempool[T]) removeFromOwned(item T) {
	sender := item.Payer()
	acct, ok := th.owned[sender]
	if !ok {
		// May no longer be populated
		return
	}
	acct.Remove(item.ID())
	if len(acct) == 0 {
		delete(th.owned, sender)
	}
}

// Has returns if the pm of [th] contains [itemID]
func (th *Mempool[T]) Has(ctx context.Context, itemID ids.ID) bool {
	_, span := th.tracer.Start(ctx, "Mempool.Has")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()
	return th.pm.Has(itemID)
}

// Add pushes all new items from [items] to th. Does not add a item if
// the item payer is not exempt and their items in the mempool exceed th.maxPayerSize.
// If the size of th exceeds th.maxSize, Add pops the lowest value item
// from th.pm.
func (th *Mempool[T]) Add(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Add")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, item := range items {
		sender := item.Payer()

		// Ensure no duplicate
		if th.pm.Has(item.ID()) {
			// Don't drop because already exists
			continue
		}

		// Optimistically add to both mempools
		acct, ok := th.owned[sender]
		if !ok {
			acct = set.Set[ids.ID]{}
			th.owned[sender] = acct
		}
		if !th.exemptPayers.Contains(sender) && acct.Len() == th.maxPayerSize {
			continue // do nothing, wait for items to expire
		}
		th.pm.Add(item)
		th.tm.Add(item)
		acct.Add(item.ID())

		// Remove the lowest paying item if at global max
		if th.pm.Len() > th.maxSize {
			// Remove the lowest paying item
			lowItem, _ := th.pm.PopMin()
			th.tm.Remove(lowItem.ID())
			th.removeFromOwned(lowItem)
		}
	}
}

// PeekMax returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PeekMax(ctx context.Context) (T, bool) {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMax")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMax()
}

// PeekMin returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PeekMin(ctx context.Context) (T, bool) {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMin")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMin()
}

// PopMax removes and returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PopMax(ctx context.Context) (T, bool) { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMax")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	max, ok := th.pm.PopMax()
	if ok {
		th.tm.Remove(max.ID())
		th.removeFromOwned(max)
	}
	return max, ok
}

// PopMin removes and returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PopMin(ctx context.Context) (T, bool) { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMin")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	min, ok := th.pm.PopMin()
	if ok {
		th.tm.Remove(min.ID())
		th.removeFromOwned(min)
	}
	return min, ok
}

// Remove removes [items] from th.
func (th *Mempool[T]) Remove(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, item := range items {
		th.pm.Remove(item.ID())
		th.tm.Remove(item.ID())
		th.removeFromOwned(item)
		// Remove is called when verifying a block. We should not drop transactions at
		// this time.
	}
}

// Len returns the number of items in th.
func (th *Mempool[T]) Len(ctx context.Context) int {
	_, span := th.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.Len()
}

// RemoveAccount removes all items by [sender] from th.
func (th *Mempool[T]) RemoveAccount(ctx context.Context, sender string) {
	_, span := th.tracer.Start(ctx, "Mempool.RemoveAccount")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.removeAccount(sender)
}

func (th *Mempool[T]) removeAccount(sender string) {
	acct, ok := th.owned[sender]
	if !ok {
		return
	}
	for item := range acct {
		th.pm.Remove(item)
		th.tm.Remove(item)
	}
	delete(th.owned, sender)
}

// SetMinTimestamp removes all items with a lower expiry than [t] from th.
// SetMinTimestamp returns the list of removed items.
func (th *Mempool[T]) SetMinTimestamp(ctx context.Context, t int64) []T {
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

func (th *Mempool[T]) Build(
	ctx context.Context,
	f func(context.Context, T) (cont bool, restore bool, removeAcct bool, err error),
) error {
	ctx, span := th.tracer.Start(ctx, "Mempool.Build")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	restorableItems := []T{}
	var err error
	for th.pm.Len() > 0 {
		max, _ := th.pm.PopMax()
		cont, restore, removeAccount, fErr := f(ctx, max)
		if restore {
			// Waiting to restore unused transactions ensures that an account will be
			// excluded from future price mempool iterations
			restorableItems = append(restorableItems, max)
		} else {
			th.tm.Remove(max.ID())
			th.removeFromOwned(max)
		}
		if removeAccount {
			// We remove the account typically when the next execution results in an
			// invalid balance
			th.removeAccount(max.Payer())
		}
		if !cont || fErr != nil {
			err = fErr
			break
		}
	}
	//
	// Restore unused items
	for _, item := range restorableItems {
		th.pm.Add(item)
	}
	return err
}
