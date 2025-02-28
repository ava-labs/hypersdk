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
	inflight atomic.Int64

	l            sync.Mutex
	confirmedTxs int
	totalTxs     int

	sent   atomic.Int64
	ticker *time.Ticker
}

func newTracker() *tracker {
	return &tracker{}
}

// logResult logs the result of a transaction received over the websocket connection
func (t *tracker) logResult(txID ids.ID, result *chain.Result) {
	t.l.Lock()
	defer t.l.Unlock()

	t.inflight.Add(-1)
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

func (t *tracker) startPeriodicLog(ctx context.Context, cli *jsonrpc.JSONRPCClient) {
	// Log stats
	t.ticker = time.NewTicker(time.Second)
	var (
		prevSent int64
		prevTime = time.Now()
	)

	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.l.Lock()
				if t.totalTxs > 0 {
					unitPrices, err := cli.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					currSent := t.sent.Load()
					currTime := time.Now()
					diff := currTime.Sub(prevTime).Seconds()
					// This should never happen, but golang only guarantees that time is monotonically increasing,
					// so we add a check to prevent division by zero here.
					if diff == 0 {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						t.totalTxs,
						float64(t.confirmedTxs)/float64(t.totalTxs)*100,
						t.inflight.Load(),
						uint64(float64(currSent-prevSent)/diff),
						unitPrices,
					)
					prevTime = currTime
					prevSent = currSent
				}
				t.l.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (t *tracker) incrementSent() int64 {
	return t.sent.Add(1)
}

func (t *tracker) incrementInflight() int64 {
	return t.inflight.Add(1)
}
