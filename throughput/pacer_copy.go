// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"
	"sync"

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

	var wg sync.WaitGroup
	errors := make(chan error, 1) 

	// Start a goroutine to process transaction results
	go func() {
		defer close(errors)
		for range p.inflight {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, wsErr, result, err := p.ws.ListenTx(ctx)
				if err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
				if wsErr != nil {
					select {
					case errors <- wsErr:
					default:
					}
					return
				}
				if !result.Success {
					select {
					case errors <- fmt.Errorf("%w: %s", ErrTxFailed, result.Error):
					default:
					}
					return
				}
			}()
		}
		
		// Wait for all goroutines to complete
		wg.Wait()
		select {
		case err := <-errors:
			utils.Outf("{{red}} Error: %s\n", err)
			p.done <- err
		default:
			p.done <- nil
		}
	}()
}

func (p *pacer) Add(tx *chain.Transaction) error {
	if err := p.ws.RegisterTx(tx); err != nil {
		return err
	}

	select {
	case p.inflight <- struct{}{}:
		return nil
	case err := <-p.done:
		utils.Outf("{{red}} Error: %s\n", err)
		return err
	}
}

func (p *pacer) Wait() error {
	close(p.inflight)
	return <-p.done
}