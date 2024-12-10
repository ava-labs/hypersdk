// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import "sync"

type Ready interface {
	Ready() bool
}

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

type GroupReady []Ready

func (g GroupReady) Ready() bool {
	for _, r := range g {
		if !r.Ready() {
			return false
		}
	}
	return true
}
