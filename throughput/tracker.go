// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
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

// logResult logs the result of a transaction received over the websocket connection
func (t *tracker) logResult(txID ids.ID, result *chain.Result) {
	t.l.Lock()
	defer t.l.Unlock()

	t.totalTxs++
	if result == nil {
		utils.Outf("{{orange}}transaction %s expired\n", txID)
		return
	}

	if result.Success {
		t.confirmedTxs++
	} else {
		utils.Outf("{{orange}}on-chain tx failure %s:{{/}} %s %t\n", txID, string(result.Error), result.Success)
	}
}

func (t *tracker) logState(ctx context.Context, cli *jsonrpc.JSONRPCClient) {
	// Log stats
	tick := time.NewTicker(1 * time.Second) // ensure no duplicates created
	var psent int64
	go func() {
		defer tick.Stop()
		prevTime := time.Now()
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
					currTime := time.Now()
					diff := min(currTime.Sub(prevTime).Seconds(), 1)
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						t.totalTxs,
						float64(t.confirmedTxs)/float64(t.totalTxs)*100,
						t.inflight.Load(),
						uint64(float64(current-psent)/diff),
						unitPrices,
					)
					prevTime = currTime
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
