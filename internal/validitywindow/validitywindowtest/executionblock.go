// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindowtest

import (
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"
)

type ExecutionBlock struct {
	Prnt   ids.ID
	Tmstmp int64
	Hght   uint64
	Ctrs   []Container
	ID     ids.ID
}

func (e ExecutionBlock) GetID() ids.ID {
	return e.ID
}

func (e ExecutionBlock) Parent() ids.ID {
	return e.Prnt
}

func (e ExecutionBlock) Timestamp() int64 {
	return e.Tmstmp
}

func (e ExecutionBlock) Height() uint64 {
	return e.Hght
}

func (e ExecutionBlock) Containers() []Container {
	return e.Ctrs
}

func (e ExecutionBlock) Contains(id ids.ID) bool {
	for _, c := range e.Ctrs {
		if c.GetID() == id {
			return true
		}
	}
	return false
}

func NewExecutionBlock(parent int64, timestamp int64, height uint64, contrainers ...int64) ExecutionBlock {
	e := ExecutionBlock{
		Prnt:   int64ToID(parent),
		Tmstmp: timestamp,
		Hght:   height,
		ID:     int64ToID(parent + 1),
	}
	for _, c := range contrainers {
		e.Ctrs = append(e.Ctrs, NewContainer(c))
	}
	return e
}

func int64ToID(n int64) ids.ID {
	var id ids.ID
	binary.LittleEndian.PutUint64(id[0:8], uint64(n))
	return id
}
