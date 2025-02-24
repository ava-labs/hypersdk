// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"
	"sync"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
)

type pacer struct {
	ws *ws.WebSocketClient

	wg sync.WaitGroup

	inflight  chan struct{}
	done      chan struct{}
	closeOnce sync.Once

	l       sync.RWMutex
	errOnce sync.Once
	err     error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan struct{}, 1)

	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case _, ok := <-p.inflight:
			if !ok {
				return
			}
			txID, result, err := p.ws.ListenTx(ctx)
			if err != nil {
				p.setError(fmt.Errorf("error listening to tx %s: %w", txID, err))
				return
			}
			if result == nil {
				p.setError(fmt.Errorf("tx %s expired", txID))
				return
			}
			if !result.Success {
				// Should never happen
				p.setError(fmt.Errorf("tx failure %w: %s", ErrTxFailed, result.Error))
				return
			}
		case <-p.done:
			return
		}
	}
}

func (p *pacer) Add(tx *chain.Transaction) error {
	p.l.RLock()
	if p.err != nil {
		defer p.l.RUnlock()
		return p.err
	}
	p.l.RUnlock()

	if err := p.ws.RegisterTx(tx); err != nil {
		p.setError(err)
		p.l.RLock()
		defer p.l.RUnlock()
		return p.err
	}
	select {
	case p.inflight <- struct{}{}:
		return nil
	case <-p.done:
		return p.err
	}
}

func (p *pacer) Wait() error {
	p.closeOnce.Do(func() {
		close(p.inflight)
	})
	p.wg.Wait()
	p.setError(nil)
	return p.err
}

func (p *pacer) setError(err error) {
	p.errOnce.Do(func() {
		p.l.Lock()
		p.err = err
		p.l.Unlock()
		close(p.done)
	})
}
