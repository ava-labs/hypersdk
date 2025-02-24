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

	inflight chan struct{}
	done     chan struct{}

	errOnce sync.Once
	err     error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan struct{}, 1)

	for {
		select {
		case _, ok := <-p.inflight:
			if !ok {
				p.setError(nil)
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
	if p.err != nil {
		return p.err
	}

	if err := p.ws.RegisterTx(tx); err != nil {
		p.setError(err)
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
	close(p.inflight)
	// Wait for all inflight transactions to finish
	<-p.done
	return p.err
}

func (p *pacer) setError(err error) {
	p.errOnce.Do(func() {
		p.err = err
		close(p.done)
	})
}
