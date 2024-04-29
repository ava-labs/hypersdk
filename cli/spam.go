// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package cli

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	defaultRange          = 32
	issuerShutdownTimeout = 60 * time.Second
)

var (
	issuerWg sync.WaitGroup
	exiting  sync.Once

	l            sync.Mutex
	confirmedTxs uint64
	totalTxs     uint64

	inflight atomic.Int64
	sent     atomic.Int64
)

func (h *Handler) Spam(
	maxTxBacklog int, maxFee *uint64, randomRecipient bool,
	createClient func(string, uint32, ids.ID) error, // must save on caller side
	getFactory func(*PrivateKey) (chain.AuthFactory, error),
	createAccount func() (*PrivateKey, error),
	lookupBalance func(int, string) (uint64, error),
	getParser func(context.Context, ids.ID) (chain.Parser, error),
	getTransfer func(codec.Address, uint64) chain.Action,
	submitDummy func(*rpc.JSONRPCClient, *PrivateKey) func(context.Context, uint64) error,
) error {
	ctx := context.Background()

	// Select chain
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}

	// Select root key
	keys, err := h.GetKeys()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return ErrNoKeys
	}
	utils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(ctx)
	if err != nil {
		return err
	}
	balances := make([]uint64, len(keys))
	if err := createClient(uris[0], networkID, chainID); err != nil {
		return err
	}
	for i := 0; i < len(keys); i++ {
		address := h.c.Address(keys[i].Address)
		balance, err := lookupBalance(i, address)
		if err != nil {
			return err
		}
		balances[i] = balance
	}
	keyIndex, err := h.PromptChoice("select root key", len(keys))
	if err != nil {
		return err
	}
	key := keys[keyIndex]
	balance := balances[keyIndex]
	factory, err := getFactory(key)
	if err != nil {
		return err
	}

	// No longer using db, so we close
	if err := h.CloseDatabase(); err != nil {
		return err
	}

	// Compute max units
	parser, err := getParser(ctx, chainID)
	if err != nil {
		return err
	}
	action := getTransfer(keys[0].Address, 0)
	maxUnits, err := chain.EstimateMaxUnits(parser.Rules(time.Now().UnixMilli()), action, factory)
	if err != nil {
		return err
	}

	// Distribute funds
	numAccounts, err := h.PromptInt("number of accounts", consts.MaxInt)
	if err != nil {
		return err
	}
	numTxsPerAccount, err := h.PromptInt("number of transactions per account per second", consts.MaxInt)
	if err != nil {
		return err
	}
	numClients, err := h.PromptInt("number of clients per node", consts.MaxInt)
	if err != nil {
		return err
	}
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err := fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	if maxFee != nil {
		feePerTx = *maxFee
		utils.Outf("{{cyan}}overriding max fee:{{/}} %d\n", feePerTx)
	}
	witholding := feePerTx * uint64(numAccounts)
	if balance < witholding {
		return fmt.Errorf("insufficient funds (have=%d need=%d)", balance, witholding)
	}
	distAmount := (balance - witholding) / uint64(numAccounts)
	utils.Outf(
		"{{yellow}}distributing funds to each account:{{/}} %s %s\n",
		utils.FormatBalance(distAmount, h.c.Decimals()),
		h.c.Symbol(),
	)
	accounts := make([]*PrivateKey, numAccounts)
	dcli, err := rpc.NewWebSocketClient(uris[0], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	funds := map[codec.Address]uint64{}
	var fundsL sync.Mutex
	for i := 0; i < numAccounts; i++ {
		// Create account
		pk, err := createAccount()
		if err != nil {
			return err
		}
		accounts[i] = pk

		// Send funds
		_, tx, err := cli.GenerateTransactionManual(parser, getTransfer(pk.Address, distAmount), factory, feePerTx)
		if err != nil {
			return err
		}
		if err := dcli.RegisterTx(tx); err != nil {
			return fmt.Errorf("%w: failed to register tx", err)
		}
		funds[pk.Address] = distAmount
	}
	for i := 0; i < numAccounts; i++ {
		_, dErr, result, err := dcli.ListenTx(ctx)
		if err != nil {
			return err
		}
		if dErr != nil {
			return dErr
		}
		if !result.Success {
			// Should never happen
			return fmt.Errorf("%w: %s", ErrTxFailed, result.Output)
		}
	}
	var recipientFunc func() (*PrivateKey, error)
	if randomRecipient {
		recipientFunc = createAccount
	}
	utils.Outf("{{yellow}}distributed funds to %d accounts{{/}}\n", numAccounts)

	// Kickoff txs
	clients := []*txIssuer{}
	for i := 0; i < len(uris); i++ {
		for j := 0; j < numClients; j++ {
			cli := rpc.NewJSONRPCClient(uris[i])
			dcli, err := rpc.NewWebSocketClient(uris[i], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				return err
			}
			clients = append(clients, &txIssuer{c: cli, d: dcli, uri: i})
		}
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// confirm txs (track failure rate)
	unitPrices, err = clients[0].c.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	PrintUnitPrices(unitPrices)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, client := range clients {
		startIssuer(cctx, client)
	}

	// log stats
	t := time.NewTicker(1 * time.Second) // ensure no duplicates created
	defer t.Stop()
	var psent int64
	go func() {
		for {
			select {
			case <-t.C:
				current := sent.Load()
				l.Lock()
				if totalTxs > 0 {
					unitPrices, err = clients[0].c.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						totalTxs,
						float64(confirmedTxs)/float64(totalTxs)*100,
						inflight.Load(),
						current-psent,
						ParseDimensions(unitPrices),
					)
				}
				l.Unlock()
				psent = current
			case <-cctx.Done():
				return
			}
		}
	}()

	// broadcast txs
	g, gctx := errgroup.WithContext(ctx)
	for ri := 0; ri < numAccounts; ri++ {
		i := ri
		g.Go(func() error {
			t := time.NewTimer(0) // ensure no duplicates created
			defer t.Stop()

			issuerIndex, issuer := getRandomIssuer(clients)
			factory, err := getFactory(accounts[i])
			if err != nil {
				return err
			}
			fundsL.Lock()
			balance := funds[accounts[i].Address]
			fundsL.Unlock()
			defer func() {
				fundsL.Lock()
				funds[accounts[i].Address] = balance
				fundsL.Unlock()
			}()
			ut := time.Now().Unix()
			for {
				select {
				case <-t.C:
					// Ensure we aren't too backlogged
					if inflight.Load() > int64(maxTxBacklog) {
						t.Reset(1 * time.Second)
						continue
					}

					// Select tx time
					//
					// Needed to prevent duplicates if called within the same
					// unix second.
					nextTime := time.Now().Unix()
					if nextTime <= ut {
						nextTime = ut + 1
					}
					ut = nextTime
					tm := &timeModifier{nextTime*consts.MillisecondsPerSecond + parser.Rules(nextTime).GetValidityWindow() - 5*consts.MillisecondsPerSecond}

					// Send transaction
					start := time.Now()
					selected := map[codec.Address]int{}
					for k := 0; k < numTxsPerAccount; k++ {
						recipient, err := getNextRecipient(i, recipientFunc, accounts)
						if err != nil {
							utils.Outf("{{orange}}failed to get next recipient:{{/}} %v\n", err)
							return err
						}
						v := selected[recipient] + 1
						selected[recipient] = v
						action := getTransfer(recipient, uint64(v))
						fee, err := fees.MulSum(unitPrices, maxUnits)
						if err != nil {
							utils.Outf("{{orange}}failed to estimate max fee:{{/}} %v\n", err)
							return err
						}
						if maxFee != nil {
							fee = *maxFee
						}
						_, tx, err := issuer.c.GenerateTransactionManual(parser, action, factory, fee, tm)
						if err != nil {
							utils.Outf("{{orange}}failed to generate tx:{{/}} %v\n", err)
							continue
						}
						if err := issuer.d.RegisterTx(tx); err != nil {
							issuer.l.Lock()
							if issuer.d.Closed() {
								if issuer.abandoned != nil {
									issuer.l.Unlock()
									return issuer.abandoned
								}
								// recreate issuer
								utils.Outf("{{orange}}re-creating issuer:{{/}} %d {{orange}}uri:{{/}} %d\n", issuerIndex, issuer.uri)
								dcli, err := rpc.NewWebSocketClient(uris[issuer.uri], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
								if err != nil {
									issuer.abandoned = err
									utils.Outf("{{orange}}could not re-create closed issuer:{{/}} %v\n", err)
									issuer.l.Unlock()
									return err
								}
								issuer.d = dcli
								startIssuer(cctx, issuer)
								issuer.l.Unlock()
								utils.Outf("{{green}}re-created closed issuer:{{/}} %d\n", issuerIndex)
							}
							continue
						}
						balance -= (fee + uint64(v))
						issuer.l.Lock()
						issuer.outstandingTxs++
						issuer.l.Unlock()
						inflight.Add(1)
						sent.Add(1)
					}

					// Determine how long to sleep
					dur := time.Since(start)
					sleep := max(float64(consts.MillisecondsPerSecond-dur.Milliseconds()), 0)
					t.Reset(time.Duration(sleep) * time.Millisecond)
				case <-gctx.Done():
					return gctx.Err()
				case <-cctx.Done():
					return nil
				case <-signals:
					exiting.Do(func() {
						utils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
						cancel()
					})
					return nil
				}
			}
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Wait for all issuers to finish
	utils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
	dctx, cancel := context.WithCancel(ctx)
	go func() {
		// Send a dummy transaction if shutdown is taking too long (listeners are
		// expired on accept if dropped)
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				utils.Outf("{{yellow}}remaining:{{/}} %d\n", inflight.Load())
				_ = h.SubmitDummy(dctx, cli, submitDummy(cli, key))
			case <-dctx.Done():
				return
			}
		}
	}()
	issuerWg.Wait()
	cancel()

	// Return funds
	unitPrices, err = cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err = fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	if maxFee != nil {
		feePerTx = *maxFee
	}
	utils.Outf("{{yellow}}returning funds to %s{{/}}\n", h.c.Address(key.Address))
	var (
		returnedBalance uint64
		returnsSent     int
	)
	for i := 0; i < numAccounts; i++ {
		balance := funds[accounts[i].Address]
		if feePerTx > balance {
			continue
		}
		returnsSent++
		// Send funds
		returnAmt := balance - feePerTx
		f, err := getFactory(accounts[i])
		if err != nil {
			return err
		}
		_, tx, err := cli.GenerateTransactionManual(parser, getTransfer(key.Address, returnAmt), f, feePerTx)
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if err := dcli.RegisterTx(tx); err != nil {
			return err
		}
		returnedBalance += returnAmt
	}
	for i := 0; i < returnsSent; i++ {
		_, dErr, result, err := dcli.ListenTx(ctx)
		if err != nil {
			return err
		}
		if dErr != nil {
			return dErr
		}
		if !result.Success {
			// Should never happen
			return fmt.Errorf("%w: %s", ErrTxFailed, result.Output)
		}
	}
	utils.Outf(
		"{{yellow}}returned funds:{{/}} %s %s\n",
		utils.FormatBalance(returnedBalance, h.c.Decimals()),
		h.c.Symbol(),
	)
	return nil
}

type txIssuer struct {
	c *rpc.JSONRPCClient
	d *rpc.WebSocketClient

	l              sync.Mutex
	uri            int
	abandoned      error
	outstandingTxs int
}

type timeModifier struct {
	Timestamp int64
}

func (t *timeModifier) Base(b *chain.Base) {
	b.Timestamp = t.Timestamp
}

func startIssuer(cctx context.Context, issuer *txIssuer) {
	issuerWg.Add(1)
	go func() {
		for {
			_, dErr, result, err := issuer.d.ListenTx(context.TODO())
			if err != nil {
				return
			}
			inflight.Add(-1)
			issuer.l.Lock()
			issuer.outstandingTxs--
			issuer.l.Unlock()
			l.Lock()
			if result != nil {
				if result.Success {
					confirmedTxs++
				} else {
					utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Output), result.Success)
				}
			} else {
				// We can't error match here because we receive it over the wire.
				if !strings.Contains(dErr.Error(), rpc.ErrExpired.Error()) {
					utils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", dErr)
				}
			}
			totalTxs++
			l.Unlock()
		}
	}()
	go func() {
		defer func() {
			_ = issuer.d.Close()
			issuerWg.Done()
		}()

		<-cctx.Done()
		start := time.Now()
		for time.Since(start) < issuerShutdownTimeout {
			if issuer.d.Closed() {
				return
			}
			issuer.l.Lock()
			outstanding := issuer.outstandingTxs
			issuer.l.Unlock()
			if outstanding == 0 {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		utils.Outf("{{orange}}issuer shutdown timeout{{/}}\n")
	}()
}

func getNextRecipient(self int, createAccount func() (*PrivateKey, error), keys []*PrivateKey) (codec.Address, error) {
	// Send to a random, new account
	if createAccount != nil {
		priv, err := createAccount()
		if err != nil {
			return codec.EmptyAddress, err
		}
		return priv.Address, nil
	}

	// Select item from array
	index := rand.Int() % len(keys)
	if index == self {
		index++
		if index == len(keys) {
			index = 0
		}
	}
	return keys[index].Address, nil
}

func getRandomIssuer(issuers []*txIssuer) (int, *txIssuer) {
	index := rand.Int() % len(issuers)
	return index, issuers[index]
}
