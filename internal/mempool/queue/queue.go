package queue

import (
	"github.com/ava-labs/avalanchego/ids"
)

type Item interface {
	GetID() ids.ID    // method for returning an id of the item
	GetExpiry() int64 // method for returning this items timestamp
}

type Queue[T Item, Elem any] interface {
	Size() int
	First() Elem
	Remove(Elem) T
	Push(T) Elem
	Restore(T) Elem
}
