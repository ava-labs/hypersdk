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
	done     chan error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan error)

	for range p.inflight {
		_, wsErr, result, err := p.ws.ListenTx(ctx)
		if err != nil {
			p.done <- err
			return
		}
		if wsErr != nil {
			p.done <- wsErr
			return
		}
		if !result.Success {
			// Should never happen
			p.done <- fmt.Errorf("%w: %s", ErrTxFailed, result.Error)
			return
		}
	}
	p.done <- nil
}

func (p *pacer) Add(tx *chain.Transaction) error {
	if err := p.ws.RegisterTx(tx); err != nil {
		return err
	}
	select {
	case p.inflight <- struct{}{}:
		return nil
	case err := <-p.done:
		return err
	}
}

func (p *pacer) Wait() error {
	close(p.inflight)
	return <-p.done
}
