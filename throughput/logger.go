// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
)

type logger struct {
	l sync.Mutex

	confirmedTxs int
	totalTxs     int
}

func (l *logger) Log(
	result *chain.Result,
	wsErr error,
) {
	l.l.Lock()
	if result != nil {
		if result.Success {
			l.confirmedTxs++
		} else {
			utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Error), result.Success)
		}
	} else {
		// We can't error match here because we receive it over the wire.
		if !strings.Contains(wsErr.Error(), ws.ErrExpired.Error()) {
			utils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", wsErr)
		}
	}
	l.totalTxs++
	l.l.Unlock()
}

func (l *logger) logStats(ctx context.Context, issuer *issuer, inflight *atomic.Int64, sent *atomic.Int64) {
	// Log stats
	t := time.NewTicker(1 * time.Second) // ensure no duplicates created
	var psent int64
	go func() {
		defer t.Stop()
		for {
			select {
			case <-t.C:
				current := sent.Load()
				l.l.Lock()
				if l.totalTxs > 0 {
					unitPrices, err := issuer.cli.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						l.totalTxs,
						float64(l.confirmedTxs)/float64(l.totalTxs)*100,
						inflight.Load(),
						current-psent,
						unitPrices,
					)
				}
				l.l.Unlock()
				psent = current
			case <-ctx.Done():
				return
			}
		}
	}()
}
