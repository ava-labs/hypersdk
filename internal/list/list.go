// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package list

import "github.com/ava-labs/avalanchego/ids"

// Item defines an interface accepted by [List].
//
// It is not necessary to include this constraint but
// allows us to avoid wrapping [Element] when using it with [emap]
// and/or [eheap].
type Item interface {
	GetID() ids.ID    // method for returning an id of the item
	GetExpiry() int64 // method for returning this items timestamp
}

// List implements a double-linked list. It offers
// similar functionality as container/list but uses
// generics.
//
// This data structure is used to implement a FIFO mempool, which
// requires arbitrary deletion of elements.
//
// Original source: https://gist.github.com/pje/90e727f80685c78a6c1cfff35f62155a
type List[T Item] struct {
	root Element[T]
	size int
}

type Element[T Item] struct {
	prev *Element[T]
	next *Element[T]
	list *List[T]

	value T
}

func (e *Element[T]) Next() *Element[T] {
	n := e.next
	if e.list == nil || n == &e.list.root {
		return nil
	}
	return n
}

func (e *Element[T]) Prev() *Element[T] {
	p := e.prev
	if e.list == nil || p == &e.list.root {
		return nil
	}
	return p
}

func (e *Element[T]) Value() T {
	return e.value
}

func (e *Element[T]) GetID() ids.ID {
	return e.value.GetID()
}

func (e *Element[T]) GetExpiry() int64 {
	return e.value.GetExpiry()
}

func (l *List[T]) First() *Element[T] {
	if l.size == 0 {
		return nil
	}
	return l.root.next
}

func (l *List[T]) Last() *Element[T] {
	if l.size == 0 {
		return nil
	}
	return l.root.prev
}

func (l *List[T]) PushFront(v T) *Element[T] {
	if l.root.next == nil {
		l.init()
	}
	return l.insertValueAfter(v, &l.root)
}

func (l *List[T]) PushBack(v T) *Element[T] {
	if l.root.next == nil {
		l.init()
	}
	return l.insertValueAfter(v, l.root.prev)
}

func (l *List[T]) Remove(e *Element[T]) T {
	if e.list == l {
		l.remove(e)
	}
	return e.value
}

func (l *List[T]) Size() int {
	return l.size
}

func (l *List[T]) init() {
	l.root = Element[T]{}
	l.root.next = &l.root
	l.root.prev = &l.root
}

func (l *List[T]) insertAfter(e *Element[T], at *Element[T]) *Element[T] {
	e.prev = at
	e.next = at.next
	e.prev.next = e
	e.next.prev = e
	e.list = l
	l.size++
	return e
}

func (l *List[T]) insertValueAfter(v T, at *Element[T]) *Element[T] {
	e := Element[T]{value: v}
	return l.insertAfter(&e, at)
}

func (l *List[T]) remove(e *Element[T]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	e.next = nil
	e.prev = nil
	e.list = nil
	l.size--
}
