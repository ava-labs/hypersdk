// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package orderbook

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/heap"
	"go.uber.org/zap"
)

const (
	initialPairCapacity = 128
	allPairs            = "*"
)

type Order struct {
	ID        ids.ID `json:"id"`
	Owner     string `json:"owner"` // we always send address over RPC
	InTick    uint64 `json:"inTick"`
	OutTick   uint64 `json:"outTick"`
	Remaining uint64 `json:"remaining"`

	owner crypto.PublicKey
}

type OrderBook struct {
	c Controller

	// TODO: consider capping the number of orders in each heap (need to ensure
	// that doing so does not make it possible to send a bunch of small, spam
	// orders to clear -> may need to set a min order limit to watch)
	orders      map[string]*heap.Heap[*Order, float64]
	orderToPair map[ids.ID]string // needed to delete from [CloseOrder] actions
	l           sync.RWMutex

	trackAll bool
}

func New(c Controller, trackedPairs []string) *OrderBook {
	m := map[string]*heap.Heap[*Order, float64]{}
	trackAll := false
	if len(trackedPairs) == 1 && trackedPairs[0] == allPairs {
		trackAll = true
		c.Logger().Info("tracking all order books")
	} else {
		for _, pair := range trackedPairs {
			// We use a max heap so we return the best rates in order.
			m[pair] = heap.New[*Order, float64](initialPairCapacity, true)
			c.Logger().Info("tracking order book", zap.String("pair", pair))
		}
	}
	return &OrderBook{
		c:           c,
		orders:      m,
		orderToPair: map[ids.ID]string{},
		trackAll:    trackAll,
	}
}

func (o *OrderBook) Add(txID ids.ID, actor crypto.PublicKey, action *actions.CreateOrder) {
	pair := actions.PairID(action.In, action.Out)
	order := &Order{
		txID,
		utils.Address(actor),
		action.InTick,
		action.OutTick,
		action.Supply,
		actor,
	}

	o.l.Lock()
	defer o.l.Unlock()
	h, ok := o.orders[pair]
	switch {
	case !ok && !o.trackAll:
		return
	case !ok && o.trackAll:
		o.c.Logger().Info("tracking order book", zap.String("pair", pair))
		h = heap.New[*Order, float64](initialPairCapacity, true)
		o.orders[pair] = h
	}
	h.Push(&heap.Entry[*Order, float64]{
		ID:    order.ID,
		Val:   float64(order.InTick) / float64(order.OutTick),
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
	entry, ok := h.Get(id) // O(log 1)
	if !ok {
		// This should never happen
		return
	}
	h.Remove(entry.Index) // O(log N)
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
	entry, ok := h.Get(id)
	if !ok {
		// This should never happen
		return
	}
	entry.Item.Remaining = remaining
}

func (o *OrderBook) Orders(pair string, limit int) []*Order {
	o.l.RLock()
	defer o.l.RUnlock()
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
