// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package orderbook

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/heap"
	"go.uber.org/zap"
)

const allPairs = "*"

type Order struct {
	ID        ids.ID `json:"id"`
	Owner     string `json:"owner"` // we always send address over RPC
	InAsset   ids.ID `json:"inAsset"`
	InTick    uint64 `json:"inTick"`
	OutAsset  ids.ID `json:"outAsset"`
	OutTick   uint64 `json:"outTick"`
	Remaining uint64 `json:"remaining"`

	owner codec.Address
}

type OrderBook struct {
	c Controller

	// Fee required to create an order should be high enough to prevent too many
	// dust orders from filling the heap.
	//
	// TODO: Allow operator to specify min creation supply per pair to be tracked
	orders           map[string]*heap.Heap[*Order, float64]
	orderToPair      map[ids.ID]string // needed to delete from [CloseOrder] actions
	maxOrdersPerPair int
	l                sync.RWMutex

	trackAll bool
}

func New(c Controller, trackedPairs []string, maxOrdersPerPair int) *OrderBook {
	m := map[string]*heap.Heap[*Order, float64]{}
	trackAll := false
	if len(trackedPairs) == 1 && trackedPairs[0] == allPairs {
		trackAll = true
		c.Logger().Info("tracking all order books")
	} else {
		for _, pair := range trackedPairs {
			// We use a max heap so we return the best rates in order.
			m[pair] = heap.New[*Order, float64](maxOrdersPerPair+1, true)
			c.Logger().Info("tracking order book", zap.String("pair", pair))
		}
	}
	return &OrderBook{
		c:                c,
		orders:           m,
		orderToPair:      map[ids.ID]string{},
		maxOrdersPerPair: maxOrdersPerPair,
		trackAll:         trackAll,
	}
}

func (o *OrderBook) Add(txID ids.ID, actor codec.Address, action *actions.CreateOrder) {
	pair := actions.PairID(action.In, action.Out)
	order := &Order{
		txID,
		codec.MustAddressBech32(consts.HRP, actor),
		action.In,
		action.InTick,
		action.Out,
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
		h = heap.New[*Order, float64](o.maxOrdersPerPair+1, true)
		o.orders[pair] = h
	}
	h.Push(&heap.Entry[*Order, float64]{
		ID:    order.ID,
		Val:   float64(order.InTick) / float64(order.OutTick),
		Item:  order,
		Index: h.Len(),
	})
	o.orderToPair[order.ID] = pair

	// Remove worst order if we are above the max we
	// track per pair
	if l := h.Len(); l > o.maxOrdersPerPair {
		e := h.Remove(l - 1)
		delete(o.orderToPair, e.ID)
	}
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
		// Clients often prefer an empty slice instead of null
		return []*Order{}
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
