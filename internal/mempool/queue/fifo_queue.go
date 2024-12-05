// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package queue

import (
	"github.com/ava-labs/hypersdk/internal/list"
)

var _ Queue[Item, *list.Element[Item]] = (*FIFOQueue[Item])(nil)

type FIFOQueue[T list.Item] struct {
	*list.List[T]
}

func NewList[T Item]() *FIFOQueue[T] {
	return &FIFOQueue[T]{
		List: &list.List[T]{},
	}
}

func (l *FIFOQueue[T]) Remove(elem *list.Element[T]) T {
	return l.List.Remove(elem)
}

func (l *FIFOQueue[T]) First() (*list.Element[T], bool) {
	first := l.List.First()
	return first, first != nil
}

func (l *FIFOQueue[T]) FirstValue() (T, bool) {
	first := l.List.First()
	if first == nil {
		return *new(T), false
	}
	return first.Value(), true
}

func (l *FIFOQueue[T]) Restore(item T) *list.Element[T] {
	return l.PushFront(item)
}

func (l *FIFOQueue[T]) Push(item T) *list.Element[T] {
	return l.PushBack(item)
}
