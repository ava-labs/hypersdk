// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/rand"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/utils"
)

type issuer struct {
	index       int
	uri         string
	parser      chain.Parser
	ruleFactory chain.RuleFactory

	ws             *ws.WebSocketClient
	outstandingTxs atomic.Int64

	// injected from the spammer
	tracker *tracker
	wg      *sync.WaitGroup
}

func newIssuer(index int, ws *ws.WebSocketClient, ruleFactory chain.RuleFactory, parser chain.Parser, uri string, tracker *tracker, wg *sync.WaitGroup) *issuer {
	return &issuer{
		index:       index,
		ws:          ws,
		ruleFactory: ruleFactory,
		parser:      parser,
		uri:         uri,
		tracker:     tracker,
		wg:          wg,
	}
}

func (i *issuer) start(ctx context.Context) {
	i.wg.Add(1)
	go func() {
		for {
			txID, result, err := i.ws.ListenTx(context.TODO())
			if err != nil {
				return
			}
			i.outstandingTxs.Add(-1)
			i.tracker.logResult(txID, result)
		}
	}()
	go func() {
		defer func() {
			_ = i.ws.Close()
			i.wg.Done()
		}()

		<-ctx.Done()
		start := time.Now()
		for time.Since(start) < issuerShutdownTimeout {
			if i.ws.Closed() {
				return
			}
			outstanding := i.outstandingTxs.Load()
			if outstanding == 0 {
				return
			}
			utils.Outf("{{orange}}waiting for issuer %d to finish:{{/}} %d\n", i.index, outstanding)
			time.Sleep(time.Second)
		}
		utils.Outf("{{orange}}issuer %d shutdown timeout{{/}}\n", i.index)
	}()
}

func (i *issuer) send(actions []chain.Action, factory chain.AuthFactory, feePerTx uint64) error {
	// Construct transaction
	rules := i.ruleFactory.GetRules(time.Now().UnixMilli())
	tx, err := chain.GenerateTransactionManual(rules, actions, factory, feePerTx)
	if err != nil {
		utils.Outf("{{orange}}failed to generate tx:{{/}} %v\n", err)
		return fmt.Errorf("failed to generate tx: %w", err)
	}

	// Register transaction
	if err := i.ws.RegisterTx(tx); err != nil {
		utils.Outf("{{orange}}failed to register tx:{{/}} %v\n", err)
		return fmt.Errorf("failed to register tx: %w", err)
	}
	i.outstandingTxs.Add(1)
	i.tracker.incrementInflight()
	return nil
}

func getRandomIssuer(issuers []*issuer) *issuer {
	index := rand.Int() % len(issuers)
	return issuers[index]
}
