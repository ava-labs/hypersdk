// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package queue

import (
	"github.com/ava-labs/hypersdk/internal/pheap"
)

var _ Queue[Item, Item] = (*PriorityQueue[Item])(nil)

type PriorityQueue[T Item] struct {
	*pheap.PriorityHeap[T]
}

func NewPriorityQueue[T Item]() *PriorityQueue[T] {
	return &PriorityQueue[T]{
		PriorityHeap: pheap.New[T](0),
	}
}

func (p *PriorityQueue[T]) Size() int {
	return p.Len()
}

func (p *PriorityQueue[T]) FirstValue() (T, bool) {
	return p.First()
}

func (p *PriorityQueue[T]) Push(item T) T {
	p.Add(item)
	return item
}

func (p *PriorityQueue[T]) Remove(item T) T {
	p.PriorityHeap.Remove(item.GetID())
	return item
}

func (p *PriorityQueue[T]) Restore(item T) T {
	p.PriorityHeap.Add(item)
	return item
}
