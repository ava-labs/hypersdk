// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pubsub

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

// connections represents a collection of connections to clients.
type Connections struct {
	lock  sync.RWMutex
	conns set.Set[*Connection]
}

// nNwConnections returns a new instance of the connections struct.
func newConnections() *Connections {
	return &Connections{}
}

// Conns returns a list of all connections in [c].
func (c *Connections) Conns() []*Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.conns.List()
}

// Remove removes [conn] from [c].
func (c *Connections) Remove(conn *Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Remove(conn)
}

// Add adds [conn] to the [c].
func (c *Connections) Add(conn *Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Add(conn)
}

// Len returns the number of connections in [c].
func (c *Connections) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.Len()
}
