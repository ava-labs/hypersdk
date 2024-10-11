// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"encoding/binary"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
)

type txProcessor struct {
	issuerWg *sync.WaitGroup
	inflight *atomic.Int64

	l            sync.Mutex
	confirmedTxs int
	totalTxs     int

	sent atomic.Int64
}

func (tp *txProcessor) logResult(
	result *chain.Result,
	wsErr error,
) {
	tp.l.Lock()
	if result != nil {
		if result.Success {
			tp.confirmedTxs++
		} else {
			utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Error), result.Success)
		}
	} else {
		// We can't error match here because we receive it over the wire.
		if !strings.Contains(wsErr.Error(), ws.ErrExpired.Error()) {
			utils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", wsErr)
		}
	}
	tp.totalTxs++
	tp.l.Unlock()
}

func (tp *txProcessor) logIssuerState(ctx context.Context, issuer *issuer) {
	// Log stats
	t := time.NewTicker(1 * time.Second) // ensure no duplicates created
	var psent int64
	go func() {
		defer t.Stop()
		for {
			select {
			case <-t.C:
				current := tp.sent.Load()
				tp.l.Lock()
				if tp.totalTxs > 0 {
					unitPrices, err := issuer.cli.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						tp.totalTxs,
						float64(tp.confirmedTxs)/float64(tp.totalTxs)*100,
						tp.inflight.Load(),
						current-psent,
						unitPrices,
					)
				}
				tp.l.Unlock()
				psent = current
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (tp *txProcessor) uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(tp.sent.Add(1)))
}
