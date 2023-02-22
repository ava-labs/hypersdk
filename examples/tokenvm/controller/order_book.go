package controller

import (
	"container/heap"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/mempool"
)

const (
	initialPairCapacity = 128
)

type Order struct {
	ID        ids.ID           `json:"id"`
	Owner     crypto.PublicKey `json:"owner"`
	Rate      uint64           `json:"rate"`
	Remaining uint64           `json:"remaining"`
}

type OrderBook struct {
	// TODO: consider capping the number of orders in each heap (need to ensure
	// that doing so does not make it possible to send a bunch of small, spam
	// orders to clear -> may need to set a min order limit to watch)
	orders      map[string]*mempool.Uint64Heap[*Order]
	orderToPair map[ids.ID]string // needed to delete from [CloseOrder] actions
	l           sync.RWMutex
}

func CreatePair(in ids.ID, out ids.ID) string {
	return fmt.Sprintf("%s-%s", in.String(), out.String())
}

func NewOrderBook(trackedPairs []string) *OrderBook {
	m := map[string]*mempool.Uint64Heap[*Order]{}
	for _, pair := range trackedPairs {
		// We use a max heap so we return the best rates in order.
		m[pair] = mempool.NewUint64Heap[*Order](initialPairCapacity, false)
	}
	return &OrderBook{
		orders:      m,
		orderToPair: map[ids.ID]string{},
	}
}

func (o *OrderBook) Add(pair string, order *Order) {
	o.l.Lock()
	defer o.l.Unlock()
	h, ok := o.orders[pair]
	if !ok {
		return
	}
	heap.Push(h, &mempool.Uint64Entry[*Order]{
		ID:    order.ID,
		Val:   order.Rate,
		Item:  order,
		Index: h.Len(),
	})
	o.orderToPair[order.ID] = pair
}

func (o *OrderBook) Remove(id ids.ID) {
	o.l.Lock()
	defer o.l.Unlock()
	pair, ok := o.orderToPair[id]
	if !ok {
		return
	}
	delete(o.orderToPair, id)
	h, ok := o.orders[pair]
	if !ok {
		// This should never happen
		return
	}
	entry, ok := h.GetID(id) // O(log 1)
	if !ok {
		// This should never happen
		return
	}
	heap.Remove(h, entry.Index) // O(log N)
}

func (o *OrderBook) UpdateRemaining(id ids.ID, remaining uint64) {
	o.l.Lock()
	defer o.l.Unlock()
	pair, ok := o.orderToPair[id]
	if !ok {
		return
	}
	h, ok := o.orders[pair]
	if !ok {
		// This should never happen
		return
	}
	entry, ok := h.GetID(id)
	if !ok {
		// This should never happen
		return
	}
	entry.Item.Remaining = remaining
}

func (o *OrderBook) Orders(pair string, limit int) []*Order {
	o.l.RLock()
	defer o.l.RLock()
	h, ok := o.orders[pair]
	if !ok {
		return nil
	}
	items := h.Items()
	arrLen := len(items)
	if limit < arrLen {
		arrLen = limit
	}
	orders := make([]*Order, arrLen)
	for i := 0; i < arrLen; i++ {
		orders[i] = items[i].Item
	}
	return orders
}
