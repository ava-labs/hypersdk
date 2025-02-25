// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
)

type pacer struct {
	ws *ws.WebSocketClient

	inflight chan struct{}
	done     chan struct{}

	err error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan struct{})

	defer close(p.done)

	for {
		select {
		case _, ok := <-p.inflight:
			if !ok {
				return
			}
			txID, result, err := p.ws.ListenTx(ctx)
			if err != nil {
				p.err = fmt.Errorf("error listening to tx %s: %w", txID, err)
				return
			}
			if result == nil {
				p.err = fmt.Errorf("tx %s expired", txID)
				return
			}
			if !result.Success {
				// Should never happen
				p.err = fmt.Errorf("tx failure %w: %s", ErrTxFailed, result.Error)
				return
			}
		case <-p.done:
			return
		}
	}
}

func (p *pacer) Add(tx *chain.Transaction) error {
	select {
	case <-p.done:
		return p.err
	default:
		if err := p.ws.RegisterTx(tx); err != nil {
			return err
		}
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
