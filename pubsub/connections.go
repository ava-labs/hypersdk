// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

// connections represents a collection of connections to clients.
type connections struct {
	lock  sync.RWMutex
	conns set.Set[*connection]
}

// newConnections returns a new instance of the connections struct.
func newConnections() *connections {
	return &connections{}
}

// Conns returns a list of all connections in [c].
func (c *connections) Conns() []*connection {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.conns.List()
}

// Remove removes [conn] from [c].
func (c *connections) Remove(conn *connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Remove(conn)
}

// Add adds [conn] to the [c].
func (c *connections) Add(conn *connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Add(conn)
}

// Len returns the number of connections in [c].
func (c *connections) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.Len()
}
