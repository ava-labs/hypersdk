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
	err      error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)

	// Start a goroutine to process transaction results
	for range p.inflight {
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
	}
}

func (p *pacer) Add(tx *chain.Transaction) error {
	if err := p.ws.RegisterTx(tx); err != nil {
		return err
	}
	select {
	case p.inflight <- struct{}{}:
		return nil
	default:
		if p.err != nil {
			utils.Outf("{{red}} Error adding tx: %s\n", p.err)
			return p.err
		}
	}
	return nil
}

// Wait should be called at most once
func (p *pacer) Wait() error {
	close(p.inflight)
	return p.err
}
