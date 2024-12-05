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

type Queue[T Item, E Item] interface {
	Size() int
	First() (E, bool)
	FirstValue() (T, bool)
	Remove(E) T
	Push(T) E
	Restore(T) E
}
