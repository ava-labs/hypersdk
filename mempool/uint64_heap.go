// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
)

type uint64Entry struct {
	id  ids.ID
	tx  *chain.Transaction
	val uint64

	index int
}

// uint64Heap is used to track pending transactions by [val]
type uint64Heap struct {
	isMinHeap bool

	items  []*uint64Entry
	lookup map[ids.ID]*uint64Entry
}

func newUint64Heap(items int, isMinHeap bool) *uint64Heap {
	return &uint64Heap{
		isMinHeap: isMinHeap,

		items:  make([]*uint64Entry, 0, items),
		lookup: make(map[ids.ID]*uint64Entry, items),
	}
}

func (th uint64Heap) Len() int { return len(th.items) }

func (th uint64Heap) Less(i, j int) bool {
	if th.isMinHeap {
		return th.items[i].val < th.items[j].val
	}
	return th.items[i].val > th.items[j].val
}

func (th uint64Heap) Swap(i, j int) {
	th.items[i], th.items[j] = th.items[j], th.items[i]
	th.items[i].index = i
	th.items[j].index = j
}

func (th *uint64Heap) Push(x interface{}) {
	entry, ok := x.(*uint64Entry)
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *uint64Entry", x))
	}
	if th.HasID(entry.id) {
		return
	}
	th.items = append(th.items, entry)
	th.lookup[entry.id] = entry
}

func (th *uint64Heap) Pop() interface{} {
	n := len(th.items)
	item := th.items[n-1]
	th.items[n-1] = nil // avoid memory leak
	th.items = th.items[0 : n-1]
	delete(th.lookup, item.id)
	return item
}

func (th *uint64Heap) GetID(id ids.ID) (*uint64Entry, bool) {
	entry, ok := th.lookup[id]
	return entry, ok
}

func (th *uint64Heap) HasID(id ids.ID) bool {
	_, has := th.GetID(id)
	return has
}
