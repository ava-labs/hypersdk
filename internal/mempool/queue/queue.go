// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package queue

import (
	"github.com/ava-labs/avalanchego/ids"
)

type Item interface {
	GetID() ids.ID    // method for returning an id of the item
	GetExpiry() int64 // method for returning this items timestamp
	Priority() uint64
}

type Queue[T Item, Elem any] interface {
	Size() int
	First() (Elem, bool)
	FirstValue() (T, bool)
	Remove(Elem) T
	Push(T) Elem
	Restore(T) Elem
}
