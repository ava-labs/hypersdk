// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
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

// NewConnections returns a new Connections instance.
func NewConnections() *Connections {
	return &Connections{}
}

// Conns returns a list of all connections in [c].
func (c *Connections) Conns() []*Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.List()
}

// Has returns if the connection [conn] is in [c].
func (c *Connections) Has(conn *Connection) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.Contains(conn)
}

// Remove removes [conn] from [c].
func (c *Connections) Remove(conn *Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conns.Remove(conn)
}

// Add adds [conn] to the [c].
//
// TODO: consider allowing same connection to register for duplicate events
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

func (c *Connections) Peek() (*Connection, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.conns.Peek()
}
