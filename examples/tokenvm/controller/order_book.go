package controller

import (
	"container/heap"
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
	orders map[string]*mempool.Uint64Heap[*Order]
	l      sync.RWMutex
}

func NewOrderBook(trackedPairs []string) *OrderBook {
	m := map[string]*mempool.Uint64Heap[*Order]{}
	for _, pair := range trackedPairs {
		// We use a max heap so we return the best rates in order.
		m[pair] = mempool.NewUint64Heap[*Order](initialPairCapacity, false)
	}
	return &OrderBook{
		orders: m,
	}
}

type OrderItem struct {
	Pair string
	Item *Order
}

func (o *OrderBook) Add(items []*OrderItem) {
	o.l.Lock()
	defer o.l.Unlock()
	for _, item := range items {
		h, ok := o.orders[item.Pair]
		if !ok {
			continue
		}
		order := item.Item
		heap.Push(h, &mempool.Uint64Entry[*Order]{
			ID:    order.ID,
			Val:   order.Rate,
			Item:  order,
			Index: h.Len(),
		})
	}
}

func (o *OrderBook) Remove(pair string, id ids.ID) {
	o.l.Lock()
	defer o.l.Unlock()
	h, ok := o.orders[pair]
	if !ok {
		return
	}
	entry, ok := h.GetID(id) // O(log 1)
	if !ok {
		return
	}
	heap.Remove(h, entry.Index) // O(log N)
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
