// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
)

type tracker struct {
	issuerWg sync.WaitGroup
	inflight atomic.Int64

	l            sync.Mutex
	confirmedTxs int
	totalTxs     int

	sent atomic.Int64
}

func (t *tracker) logResult(
	result *chain.Result,
	wsErr error,
) {
	t.l.Lock()
	if result != nil {
		if result.Success {
			t.confirmedTxs++
		} else {
			utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Error), result.Success)
		}
	} else {
		// We can't error match here because we receive it over the wire.
		if !strings.Contains(wsErr.Error(), ws.ErrExpired.Error()) {
			utils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", wsErr)
		}
	}
	t.totalTxs++
	t.l.Unlock()
}

func (t *tracker) logState(ctx context.Context, cli *jsonrpc.JSONRPCClient) {
	// Log stats
	tick := time.NewTicker(1 * time.Second) // ensure no duplicates created
	var psent int64
	go func() {
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				current := t.sent.Load()
				t.l.Lock()
				if t.totalTxs > 0 {
					unitPrices, err := cli.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						t.totalTxs,
						float64(t.confirmedTxs)/float64(t.totalTxs)*100,
						t.inflight.Load(),
						current-psent,
						unitPrices,
					)
				}
				t.l.Unlock()
				psent = current
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (t *tracker) IncrementSent() int64 {
	return t.sent.Add(1)
}
