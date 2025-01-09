// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindowtest

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/internal/emap"
)

var _ emap.Item = (*Container)(nil)

type Container struct {
	ID     ids.ID
	Expiry int64
}

// method for returning an id of the item
func (c Container) GetID() ids.ID {
	return c.ID
}

// method for returning this items timestamp
func (c Container) GetExpiry() int64 {
	return c.Expiry
}

func NewContainer(expiry int64) Container {
	return Container{
		Expiry: expiry,
		ID:     int64ToID(expiry),
	}
}
