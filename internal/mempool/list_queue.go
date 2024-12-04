package mempool

import (
	"github.com/ava-labs/hypersdk/internal/list"
	"github.com/ava-labs/hypersdk/internal/mempool/queue"
)

var _ queue.Queue[list.Item, *list.Element[list.Item]] = (*List[list.Item])(nil)

type List[T list.Item] struct {
	*list.List[T]
}

func NewList[T Item]() *List[T] {
	return &List[T]{
		List: &list.List[T]{},
	}
}

func (l *List[T]) Remove(elem *list.Element[T]) T {
	return l.List.Remove(elem)
}

func (l *List[T]) First() *list.Element[T] {
	return l.List.First()
}

func (l *List[T]) Restore(item T) *list.Element[T] {
	return l.PushFront(item)
}

func (l *List[T]) Push(item T) *list.Element[T] {
	return l.PushBack(item)
}
