// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
)

type pacer struct {
	ws *ws.WebSocketClient

	inflight chan struct{}
	done     chan error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan error, 1)

	// Start a goroutine to process transaction results
	for range p.inflight {
		_, wsErr, result, err := p.ws.ListenTx(ctx)
		if err != nil {
			p.done <- fmt.Errorf("error listening to tx: %w", err)
			return
		}
		if wsErr != nil {
			p.done <- fmt.Errorf("websocket error: %w", wsErr)
			return
		}
		if !result.Success {
			// Should never happen
			p.done <- fmt.Errorf("tx failure %w: %s", ErrTxFailed, result.Error)
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
		utils.Outf("{{red}} Error adding tx: %s\n", err)
		return err
	}
}

func (p *pacer) Wait() error {
	close(p.inflight)
	return <-p.done
}
