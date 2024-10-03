package loadgen

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
	"golang.org/x/exp/rand"
)

type issuer struct {
	i      int
	uri    string
	parser chain.Parser

	// TODO: clean up potential race conditions here.
	l              sync.Mutex
	cli            *jsonrpc.JSONRPCClient
	ws             *ws.WebSocketClient
	outstandingTxs int
	abandoned      error
}

func (i *issuer) Start(ctx context.Context) {
	issuerWg.Add(1)
	go func() {
		for {
			_, wsErr, result, err := i.ws.ListenTx(context.TODO())
			if err != nil {
				return
			}
			i.l.Lock()
			i.outstandingTxs--
			i.l.Unlock()
			inflight.Add(-1)
			l.Lock()
			if result != nil {
				if result.Success {
					confirmedTxs++
				} else {
					utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Error), result.Success)
				}
			} else {
				// We can't error match here because we receive it over the wire.
				if !strings.Contains(wsErr.Error(), ws.ErrExpired.Error()) {
					utils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", wsErr)
				}
			}
			totalTxs++
			l.Unlock()
		}
	}()
	go func() {
		defer func() {
			_ = i.ws.Close()
			issuerWg.Done()
		}()

		<-ctx.Done()
		start := time.Now()
		for time.Since(start) < issuerShutdownTimeout {
			if i.ws.Closed() {
				return
			}
			i.l.Lock()
			outstanding := i.outstandingTxs
			i.l.Unlock()
			if outstanding == 0 {
				return
			}
			utils.Outf("{{orange}}waiting for issuer %d to finish:{{/}} %d\n", i.i, outstanding)
			time.Sleep(time.Second)
		}
		utils.Outf("{{orange}}issuer %d shutdown timeout{{/}}\n", i.i)
	}()
}

func (i *issuer) Send(ctx context.Context, actions []chain.Action, factory chain.AuthFactory, feePerTx uint64) error {
	// Construct transaction
	_, tx, err := i.cli.GenerateTransactionManual(i.parser, actions, factory, feePerTx)
	if err != nil {
		utils.Outf("{{orange}}failed to generate tx:{{/}} %v\n", err)
		return fmt.Errorf("failed to generate tx: %w", err)
	}

	// Increase outstanding txs for issuer
	i.l.Lock()
	i.outstandingTxs++
	i.l.Unlock()
	inflight.Add(1)

	// Register transaction and recover upon failure
	if err := i.ws.RegisterTx(tx); err != nil {
		i.l.Lock()
		if i.ws.Closed() {
			if i.abandoned != nil {
				i.l.Unlock()
				return i.abandoned
			}

			// Attempt to recreate issuer
			utils.Outf("{{orange}}re-creating issuer:{{/}} %d {{orange}}uri:{{/}} %s\n", i.i, i.uri)
			ws, err := ws.NewWebSocketClient(i.uri, ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				i.abandoned = err
				utils.Outf("{{orange}}could not re-create closed issuer:{{/}} %v\n", err)
				i.l.Unlock()
				return err
			}
			i.ws = ws
			i.l.Unlock()

			i.Start(ctx)
			utils.Outf("{{green}}re-created closed issuer:{{/}} %d\n", i.i)
		}

		// If issuance fails during retry, we should fail
		return i.ws.RegisterTx(tx)
	}
	return nil
}

func getRandomIssuer(issuers []*issuer) *issuer {
	index := rand.Int() % len(issuers)
	return issuers[index]
}
