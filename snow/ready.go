// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"sync"
)

type Ready interface {
	Ready() bool
}

// ChanReady implements the Ready interface with a channel that
// can be marked ready by the caller.
type ChanReady struct {
	readyOnce sync.Once
	ready     chan struct{}
}

func NewChanReady() *ChanReady {
	return &ChanReady{ready: make(chan struct{})}
}

func (c *ChanReady) Ready() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

func (c *ChanReady) MarkReady() {
	c.readyOnce.Do(func() { close(c.ready) })
}

type GroupReady struct {
	lock  sync.RWMutex
	ready bool
	rs    []Ready
}

func NewGroupReady() *GroupReady {
	return &GroupReady{}
}

// Add attempts to add the provided Ready instances to the group.
// Returns false if the instances could not be added because the
// group was already marked ready.
func (g *GroupReady) Add(r ...Ready) bool {
	g.lock.Lock()
	defer g.lock.Unlock()

	if g.ready {
		return false
	}
	g.rs = append(g.rs, r...)
	return true
}

func (g *GroupReady) Ready() bool {
	g.lock.RLock()
	defer g.lock.RUnlock()

	if g.ready {
		return true
	}
	for _, r := range g.rs {
		if !r.Ready() {
			return false
		}
	}
	g.ready = true
	return true
}
