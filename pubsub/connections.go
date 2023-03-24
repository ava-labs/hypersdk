// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

type connections struct {
	lock  sync.RWMutex
	conns set.Set[*connection]
}

func newConnections() *connections {
	return &connections{}
}

func (c *connections) Conns() []*connection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.List()
}

func (c *connections) Remove(conn *connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Remove(conn)
}

func (c *connections) Add(conn *connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Add(conn)
}

func (c *connections) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.Len()
}
