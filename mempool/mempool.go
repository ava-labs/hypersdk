// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.opentelemetry.io/otel/attribute"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/list"
)

const maxPrealloc = 4_096

type Item interface {
	eheap.Item

	Sponsor() codec.Address
	Size() int
}

type Mempool[T Item] struct {
	tracer trace.Tracer

	mu sync.RWMutex

	pendingSize int // bytes

	maxSize        int
	maxSponsorSize int // Maximum items allowed by a single sponsor

	queue *list.List[T]
	eh    *eheap.ExpiryHeap[*list.Element[T]]

	// owned tracks the number of items in the mempool owned by a single
	// [Sponsor]
	owned map[codec.Address]int

	// streamedItems have been removed from the mempool during streaming
	// and should not be re-added by calls to [Add].
	streamLock        sync.Mutex // should never be needed
	streamedItems     set.Set[ids.ID]
	nextStream        []T
	nextStreamFetched bool

	// sponsors that are exempt from [maxSponsorSize]
	exemptSponsors set.Set[codec.Address]
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New[T Item](
	tracer trace.Tracer,
	maxSize int, // items
	maxSponsorSize int,
	exemptSponsors []codec.Address,
) *Mempool[T] {
	m := &Mempool[T]{
		tracer: tracer,

		maxSize:        maxSize,
		maxSponsorSize: maxSponsorSize,

		queue: &list.List[T]{},
		eh:    eheap.New[*list.Element[T]](min(maxSize, maxPrealloc)),

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

	return m.eh.Has(itemID)
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
		if m.eh.Has(itemID) {
			// Don't drop because already exists
			continue
		}

		// Ensure sender isn't abusing mempool
		senderItems := m.owned[sender]
		if !m.exemptSponsors.Contains(sender) && senderItems == m.maxSponsorSize {
			continue // do nothing, wait for items to expire
		}

		// Ensure mempool isn't full
		if m.queue.Size() == m.maxSize {
			continue // do nothing, wait for items to expire
		}

		// Add to mempool
		var elem *list.Element[T]
		if !front {
			elem = m.queue.PushBack(item)
		} else {
			elem = m.queue.PushFront(item)
		}
		m.eh.Add(elem)
		m.owned[sender]++
		m.pendingSize += item.Size()
	}
}

// PeekNext returns the highest valued item in m.eh.
// Assumes there is non-zero items in [Mempool]
func (m *Mempool[T]) PeekNext(ctx context.Context) (T, bool) {
	_, span := m.tracer.Start(ctx, "Mempool.PeekNext")
	defer span.End()

	m.mu.RLock()
	defer m.mu.RUnlock()

	first := m.queue.First()
	if first == nil {
		return *new(T), false
	}
	return first.Value(), true
}

// PopNext removes and returns the highest valued item in m.eh.
// Assumes there is non-zero items in [Mempool]
func (m *Mempool[T]) PopNext(ctx context.Context) (T, bool) { // O(log N)
	_, span := m.tracer.Start(ctx, "Mempool.PopNext")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.popNext()
}

func (m *Mempool[T]) popNext() (T, bool) {
	first := m.queue.First()
	if first == nil {
		return *new(T), false
	}
	v := m.queue.Remove(first)
	m.eh.Remove(v.ID())
	m.removeFromOwned(v)
	m.pendingSize -= v.Size()
	return v, true
}

// Remove removes [items] from m.
func (m *Mempool[T]) Remove(ctx context.Context, items []T) {
	_, span := m.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		elem, ok := m.eh.Remove(item.ID())
		if !ok {
			continue
		}
		m.queue.Remove(elem)
		m.removeFromOwned(item)
		m.pendingSize -= item.Size()
	}
}

// Len returns the number of items in m.
func (m *Mempool[T]) Len(ctx context.Context) int {
	_, span := m.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.eh.Len()
}

// Size returns the size (in bytes) of items in m.
func (m *Mempool[T]) Size(context.Context) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.pendingSize
}

// SetMinTimestamp removes and returns all items with a lower expiry than [t] from m.
func (m *Mempool[T]) SetMinTimestamp(ctx context.Context, t int64) []T {
	_, span := m.tracer.Start(ctx, "Mempool.SetMinTimesamp")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	removedElems := m.eh.SetMin(t)
	removed := make([]T, len(removedElems))
	for i, remove := range removedElems {
		m.queue.Remove(remove)
		v := remove.Value()
		m.removeFromOwned(v)
		m.pendingSize -= v.Size()
		removed[i] = v
	}
	return removed
}

// Top iterates over the highest-valued items in the mempool.
func (m *Mempool[T]) Top(
	ctx context.Context,
	targetDuration time.Duration,
	f func(context.Context, T) (cont bool, restore bool, err error),
) error {
	ctx, span := m.tracer.Start(ctx, "Mempool.Top")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	var (
		start           = time.Now()
		restorableItems = []T{}
		err             error
	)
	for m.eh.Len() > 0 {
		next, _ := m.popNext()
		cont, restore, fErr := f(ctx, next)
		if restore {
			// Waiting to restore unused transactions ensures that an account will be
			// excluded from future price mempool iterations
			restorableItems = append(restorableItems, next)
		}
		if !cont || time.Since(start) > targetDuration || fErr != nil {
			err = fErr
			break
		}
	}

	// Restore unused items
	m.add(restorableItems, true)
	return err
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
	m.streamedItems = set.NewSet[ids.ID](maxPrealloc)
}

// PrepareStream prefetches the next [count] items from the mempool to
// reduce the latency of calls to [StreamItems].
func (m *Mempool[T]) PrepareStream(ctx context.Context, count int) {
	_, span := m.tracer.Start(ctx, "Mempool.PrepareStream")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextStream = m.streamItems(count)
	m.nextStreamFetched = true
}

// Stream gets the next highest-valued [count] items from the mempool, not
// including what has already been streamed.
func (m *Mempool[T]) Stream(ctx context.Context, count int) []T {
	_, span := m.tracer.Start(ctx, "Mempool.Stream")
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nextStreamFetched {
		txs := m.nextStream
		m.nextStream = nil
		m.nextStreamFetched = false
		return txs
	}
	return m.streamItems(count)
}

func (m *Mempool[T]) streamItems(count int) []T {
	txs := make([]T, 0, count)
	for len(txs) < count {
		item, ok := m.popNext()
		if !ok {
			break
		}
		m.streamedItems.Add(item.ID())
		txs = append(txs, item)
	}
	return txs
}

// FinishStreaming restores [restorable] items to the mempool and clears
// the set of all previously streamed items.
func (m *Mempool[T]) FinishStreaming(ctx context.Context, restorable []T) int {
	_, span := m.tracer.Start(ctx, "Mempool.FinishStreaming")
	defer span.End()

	span.SetAttributes(
		attribute.Int("restorable", len(restorable)),
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	restored := len(restorable)
	m.streamedItems = nil
	m.add(restorable, true)
	if m.nextStreamFetched {
		m.add(m.nextStream, true)
		restored += len(m.nextStream)
		m.nextStream = nil
		m.nextStreamFetched = false
	}
	m.streamLock.Unlock()
	return restored
}
