// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/list"
	"go.opentelemetry.io/otel/attribute"
)

const maxPrealloc = 4_096

type Item interface {
	eheap.Item

	Payer() string
}

type Mempool[T Item] struct {
	tracer trace.Tracer

	mu sync.RWMutex

	maxSize      int
	maxPayerSize int // Maximum items allowed by a single payer

	queue *list.List[T]
	eh    *eheap.ExpiryHeap[*list.Element[T]]

	// owned tracks the number of items in the mempool owned by a single
	// [Payer]
	owned map[string]int

	// streamedItems have been removed from the mempool during streaming
	// and should not be re-added by calls to [Add].
	streamLock        sync.Mutex // should never be needed
	streamedItems     set.Set[ids.ID]
	nextStream        []T
	nextStreamFetched bool

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

		queue: &list.List[T]{},
		eh:    eheap.New[*list.Element[T]](math.Min(maxSize, maxPrealloc)),

		owned:        map[string]int{},
		exemptPayers: set.Set[string]{},
	}
	for _, payer := range exemptPayers {
		m.exemptPayers.Add(string(payer))
	}
	return m
}

func (th *Mempool[T]) removeFromOwned(item T) {
	sender := item.Payer()
	items, ok := th.owned[sender]
	if !ok {
		// May no longer be populated
		return
	}
	if items == 1 {
		delete(th.owned, sender)
		return
	}
	th.owned[sender] = items - 1
}

// Has returns if the pm of [th] contains [itemID]
func (th *Mempool[T]) Has(ctx context.Context, itemID ids.ID) bool {
	_, span := th.tracer.Start(ctx, "Mempool.Has")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	return th.eh.Has(itemID)
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

	th.add(items, false)
}

func (th *Mempool[T]) add(items []T, front bool) {
	for _, item := range items {
		sender := item.Payer()

		// Ensure no duplicate
		itemID := item.ID()
		if th.streamedItems != nil && th.streamedItems.Contains(itemID) {
			continue
		}
		if th.eh.Has(itemID) {
			// Don't drop because already exists
			continue
		}

		// Ensure sender isn't abusing mempool
		senderItems := th.owned[sender]
		if !th.exemptPayers.Contains(sender) && senderItems == th.maxPayerSize {
			continue // do nothing, wait for items to expire
		}

		// Ensure mempool isn't full
		if th.queue.Size() == th.maxSize {
			continue // do nothing, wait for items to expire
		}

		// Add to mempool
		var elem *list.Element[T]
		if !front {
			elem = th.queue.PushBack(item)
		} else {
			elem = th.queue.PushFront(item)
		}
		th.eh.Add(elem)
		th.owned[sender]++
	}
}

// PeekNext returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PeekNext(ctx context.Context) (T, bool) {
	_, span := th.tracer.Start(ctx, "Mempool.PeekNext")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	first := th.queue.First()
	if first == nil {
		return *new(T), false
	}
	return first.Value, true
}

// PopNext removes and returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PopNext(ctx context.Context) (T, bool) { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopNext")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	return th.popNext()
}

func (th *Mempool[T]) popNext() (T, bool) {
	first := th.queue.First()
	if first == nil {
		return *new(T), false
	}
	v := th.queue.Remove(first)
	th.eh.Remove(v.ID())
	th.removeFromOwned(v)
	return v, true
}

// Remove removes [items] from th.
func (th *Mempool[T]) Remove(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, item := range items {
		elem, ok := th.eh.Remove(item.ID())
		if !ok {
			continue
		}
		th.queue.Remove(elem)
		th.removeFromOwned(item)
	}
}

// Len returns the number of items in th.
func (th *Mempool[T]) Len(ctx context.Context) int {
	_, span := th.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.eh.Len()
}

// SetMinTimestamp removes all items with a lower expiry than [t] from th.
// SetMinTimestamp returns the list of removed items.
func (th *Mempool[T]) SetMinTimestamp(ctx context.Context, t int64) []T {
	_, span := th.tracer.Start(ctx, "Mempool.SetMinTimesamp")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	removedElems := th.eh.SetMinVal(t)
	removed := make([]T, len(removedElems))
	for i, remove := range removedElems {
		th.queue.Remove(remove)
		th.removeFromOwned(remove.Value)
		removed[i] = remove.Value
	}
	return removed
}

// Top iterates over the highest-valued items in the mempool.
func (th *Mempool[T]) Top(
	ctx context.Context,
	targetDuration time.Duration,
	f func(context.Context, T) (cont bool, restore bool, err error),
) error {
	ctx, span := th.tracer.Start(ctx, "Mempool.Top")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	var (
		start           = time.Now()
		restorableItems = []T{}
		err             error
	)
	for th.eh.Len() > 0 {
		next, _ := th.popNext()
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
	th.add(restorableItems, true)
	return err
}

// StartStreaming allows for async iteration over the highest-value items
// in the mempool. When done streaming, invoke [FinishStreaming] with all
// items that should be restored.
//
// Streaming is useful for block building because we can get a feed of the
// best txs to build without holding the lock during the duration of the build
// process. Streaming in batches allows for various state prefetching operations.
func (th *Mempool[T]) StartStreaming(_ context.Context) {
	th.mu.Lock()
	defer th.mu.Unlock()

	th.streamLock.Lock()
	th.streamedItems = set.NewSet[ids.ID](maxPrealloc)
}

// PrepareStream prefetches the next [count] items from the mempool to
// reduce the latency of calls to [StreamItems].
func (th *Mempool[T]) PrepareStream(ctx context.Context, count int) {
	_, span := th.tracer.Start(ctx, "Mempool.PrepareStream")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.nextStream = th.streamItems(count)
	th.nextStreamFetched = true
}

// Stream gets the next highest-valued [count] items from the mempool, not
// including what has already been streamed.
func (th *Mempool[T]) Stream(ctx context.Context, count int) []T {
	_, span := th.tracer.Start(ctx, "Mempool.Stream")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	if th.nextStreamFetched {
		txs := th.nextStream
		th.nextStream = nil
		th.nextStreamFetched = false
		return txs
	}
	return th.streamItems(count)
}

func (th *Mempool[T]) streamItems(count int) []T {
	txs := make([]T, 0, count)
	for len(txs) < count {
		item, ok := th.popNext()
		if !ok {
			break
		}
		th.streamedItems.Add(item.ID())
		txs = append(txs, item)
	}
	return txs
}

// FinishStreaming restores [restorable] items to the mempool and clears
// the set of all previously streamed items.
func (th *Mempool[T]) FinishStreaming(ctx context.Context, restorable []T) int {
	_, span := th.tracer.Start(ctx, "Mempool.FinishStreaming")
	defer span.End()

	span.SetAttributes(
		attribute.Int("restorable", len(restorable)),
	)

	th.mu.Lock()
	defer th.mu.Unlock()

	restored := len(restorable)
	th.streamedItems = nil
	th.add(restorable, true)
	if th.nextStreamFetched {
		th.add(th.nextStream, true)
		restored += len(th.nextStream)
		th.nextStream = nil
		th.nextStreamFetched = false
	}
	th.streamLock.Unlock()
	return restored
}
