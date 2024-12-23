// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package lifecycle

import (
	"sync"
	"sync/atomic"
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

func (c *ChanReady) AwaitReady(done <-chan struct{}) bool {
	select {
	case <-c.ready:
		return true
	case <-done:
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

type AtomicBoolReady struct {
	b atomic.Bool
}

func NewAtomicBoolReady(initialState bool) *AtomicBoolReady {
	a := &AtomicBoolReady{}
	a.b.Store(initialState)
	return a
}

func (a *AtomicBoolReady) Ready() bool {
	return a.b.Load()
}

func (a *AtomicBoolReady) MarkReady() {
	a.b.Store(true)
}

func (a *AtomicBoolReady) MarkNotReady() {
	a.b.Store(false)
}
