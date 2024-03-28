// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/oexpirer"
)

const (
	preallocItems       = 128_000
	streamPreallocItems = 16_384
)

type Item interface {
	ID() ids.ID
	Expiry() int64

	Sponsor() codec.Address
	Size() int
}

type Mempool[T Item] struct {
	tracer trace.Tracer

	mu sync.RWMutex

	size           int // bytes
	maxSize        int // bytes
	maxSponsorSize int // Maximum items allowed by a single sponsor

	// txs
	txs *oexpirer.OExpirer[T]

	// owned tracks the number of items in the mempool owned by a single
	// [Sponsor]
	owned map[codec.Address]int

	// sponsors that are exempt from [maxSponsorSize]
	exemptSponsors set.Set[codec.Address]

	// streamedItems have been removed from the mempool during streaming
	// and should not be re-added by calls to [Add].
	streamLock    sync.Mutex // should never be needed
	streamedItems set.Set[ids.ID]
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New[T Item](
	tracer trace.Tracer,
	maxSize int, // bytes
	maxSponsorSize int,
	exemptSponsors []codec.Address,
) *Mempool[T] {
	m := &Mempool[T]{
		tracer: tracer,

		maxSize:        maxSize,
		maxSponsorSize: maxSponsorSize,

		txs: oexpirer.New[T](preallocItems),

		owned:          map[codec.Address]int{},
		exemptSponsors: set.Set[codec.Address]{},
	}
	for _, sponsor := range exemptSponsors {
		m.exemptSponsors.Add(sponsor)
	}
	return m
}

func (m *Mempool[T]) removeFromOwned(item T) {
	sender := item.Sponsor()
	items, ok := m.owned[sender]
	if !ok {
		// May no longer be populated
		return
	}
	if items == 1 {
		delete(m.owned, sender)
		return
	}
	m.owned[sender] = items - 1
}

// Has returns if the eh of [m] contains [itemID]
func (m *Mempool[T]) Has(ctx context.Context, itemID ids.ID) bool {
	_, span := m.tracer.Start(ctx, "Mempool.Has")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.txs.Has(itemID)
}

// Add pushes all new items from [items] to m. Does not add a item if
// the item sponsor is not exempt and their items in the mempool exceed m.maxSponsorSize.
// If the size of m exceeds m.maxSize, Add pops the lowest value item
// from m.eh.
func (m *Mempool[T]) Add(ctx context.Context, items []T) {
	_, span := m.tracer.Start(ctx, "Mempool.Add")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.add(items, false)
}

func (m *Mempool[T]) add(items []T, front bool) {
	for _, item := range items {
		sender := item.Sponsor()

		// Ensure no duplicate
		itemID := item.ID()
		if m.streamedItems != nil && m.streamedItems.Contains(itemID) {
			continue
		}
		if m.txs.Has(itemID) {
			// Don't drop because already exists
			continue
		}

		// We are willing to go over the limit if we are restoring
		// to the front of the mempool (didn't finish building).
		if !front {
			// Ensure sender isn't abusing mempool
			senderItems := m.owned[sender]
			if !m.exemptSponsors.Contains(sender) && senderItems == m.maxSponsorSize {
				continue // do nothing, wait for items to expire
			}

			// Ensure mempool isn't full
			if m.size+item.Size() >= m.maxSize {
				continue // do nothing, wait for items to expire
			}
		}

		// Add to mempool
		m.txs.Add(item, front)
		m.owned[sender]++
		m.size += item.Size()
	}
}

// Remove removes [items] from m.
func (m *Mempool[T]) Remove(ctx context.Context, items []T) {
	_, span := m.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		elem, ok := m.txs.Remove(item.ID())
		if !ok {
			continue
		}
		m.removeFromOwned(elem)
		m.size -= item.Size()
	}
}

// Len returns the number of items in m.
func (m *Mempool[T]) Len(ctx context.Context) int {
	_, span := m.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	return m.txs.Len()
}

// Size returns the size (in bytes) of items in m.
func (m *Mempool[T]) Size(ctx context.Context) int {
	_, span := m.tracer.Start(ctx, "Mempool.Size")
	defer span.End()

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.size
}

// SetMinTimestamp removes and returns all items with a lower expiry than [t] from m.
func (m *Mempool[T]) SetMinTimestamp(ctx context.Context, t int64) []T {
	_, span := m.tracer.Start(ctx, "Mempool.SetMinTimesamp")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	removedElems := m.txs.SetMin(t)
	removed := make([]T, len(removedElems))
	for i, v := range removedElems {
		m.removeFromOwned(v)
		m.size -= v.Size()
		removed[i] = v
	}
	return removed
}

// StartStreaming allows for async iteration over the highest-value items
// in the mempool. When done streaming, invoke [FinishStreaming] with all
// items that should be restored.
//
// Streaming is useful for block building because we can get a feed of the
// best txs to build without holding the lock during the duration of the build
// process. Streaming in batches allows for various state prefetching operations.
func (m *Mempool[T]) StartStreaming(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.streamLock.Lock()
	m.streamedItems = set.NewSet[ids.ID](streamPreallocItems)
}

// Stream gets the next highest-valued [count] items from the mempool, not
// including what has already been streamed.
func (m *Mempool[T]) Stream(ctx context.Context) (T, bool) {
	_, span := m.tracer.Start(ctx, "Mempool.Stream")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.txs.RemoveNext()
	if !ok {
		return *new(T), false
	}
	m.removeFromOwned(item)
	m.size -= item.Size()
	m.streamedItems.Add(item.ID())
	return item, true
}

// FinishStreaming restores [restorable] items to the mempool and clears
// the set of all previously streamed items.
func (m *Mempool[T]) FinishStreaming(ctx context.Context, restorable []T) {
	_, span := m.tracer.Start(ctx, "Mempool.FinishStreaming")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.streamedItems = nil
	m.add(restorable, true)
	m.streamLock.Unlock()
}
