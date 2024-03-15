// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package cli

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

const (
	defaultRange          = 32
	issuerShutdownTimeout = 60 * time.Second

	plotSamples    = 1000
	plotIdentities = 10
	plotBarWidth   = 10
	plotOverhead   = 10
	plotHeight     = 25
)

// TODO: we should NEVER use globals, remove this
var (
	maxConcurrency = runtime.NumCPU()

	issuerWg sync.WaitGroup
	exiting  sync.Once

	l            sync.Mutex
	confirmedTxs uint64
	expiredTxs   uint64
	totalTxs     uint64

	issuedTxs atomic.Int64
	inflight  atomic.Int64
	sent      atomic.Int64
	bytes     atomic.Int64
)

func (h *Handler) Spam(
	maxTxBacklog int, numAccounts int, txsPerSecond int,
	sZipf float64, vZipf float64, plotZipf bool,
	numClients int, clusterInfo string, privateKey *PrivateKey,
	createClient func(string, uint32, ids.ID) error, // must save on caller side
	getFactory func(*PrivateKey) (chain.AuthFactory, error),
	createAccount func() (*PrivateKey, error),
	lookupBalance func(int, string) (uint64, error),
	getParser func(context.Context, ids.ID) (chain.Parser, error),
	getTransfer func(codec.Address, uint64, []byte) chain.Action, // []byte prevents duplicate txs
	submitDummy func(*rpc.JSONRPCClient, *PrivateKey) func(context.Context, uint64) error,
) error {
	ctx := context.Background()

	// Plot Zipf
	if plotZipf {
		if err := ui.Init(); err != nil {
			return err
		}
		zb := rand.NewZipf(rand.New(rand.NewSource(0)), sZipf, vZipf, plotIdentities-1) //nolint:gosec
		distribution := make([]float64, plotIdentities)
		for i := 0; i < plotSamples; i++ {
			distribution[zb.Uint64()]++
		}
		labels := make([]string, plotIdentities)
		for i := 0; i < plotIdentities; i++ {
			labels[i] = fmt.Sprintf("%d", i)
		}
		bc := widgets.NewBarChart()
		bc.Title = fmt.Sprintf("Account Issuance Distribution (s=%.2f v=%.2f [(v+k)^(-s)])", sZipf, vZipf)
		bc.Data = distribution
		bc.Labels = labels
		bc.BarWidth = plotBarWidth
		bc.SetRect(0, 0, plotOverhead+plotBarWidth*plotIdentities, plotHeight)
		ui.Render(bc)
		utils.Outf("\ntype any character to continue...\n")
		for range ui.PollEvents() {
			break
		}
		ui.Close()
	}

	// Select chain
	var (
		chainID ids.ID
		uris    map[string]string
		err     error
	)
	if len(clusterInfo) == 0 {
		chainID, uris, err = h.PromptChain("select chainID", nil)
	} else {
		chainID, uris, err = ReadCLIFile(clusterInfo)
	}
	if err != nil {
		return err
	}
	uriNames := onlyAPIs(uris)
	baseName := uriNames[0]

	// Select root key
	cli := rpc.NewJSONRPCClient(uris[baseName])
	networkID, _, _, err := cli.Network(ctx)
	if err != nil {
		return err
	}
	if err := createClient(uris[baseName], networkID, chainID); err != nil {
		return err
	}
	var (
		key     *PrivateKey
		balance uint64
	)
	if privateKey == nil {
		keys, err := h.GetKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return ErrNoKeys
		}
		utils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
		balances := make([]uint64, len(keys))
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
		key = keys[keyIndex]
		balance = balances[keyIndex]
	} else {
		balance, err = lookupBalance(-1, h.c.Address(privateKey.Address))
		if err != nil {
			return err
		}
		key = privateKey
	}
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
	action := getTransfer(key.Address, 0, uniqueBytes())
	maxUnits, err := chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), action, factory, nil)
	if err != nil {
		return err
	}

	// Distribute funds
	if numAccounts == 0 {
		numAccounts, err = h.PromptInt("number of accounts", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if txsPerSecond == 0 {
		txsPerSecond, err = h.PromptInt("txs to issue per second", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if numClients == 0 {
		numClients, err = h.PromptInt("number of clients per node", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err := chain.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	witholding := feePerTx * uint64(numAccounts)
	if balance < witholding {
		return fmt.Errorf("insufficient funds (have=%d need=%d)", balance, witholding)
	}
	distAmount := (balance - witholding) / uint64(numAccounts)
	utils.Outf(
		"{{yellow}}distributing funds to accounts (1k batch):{{/}} %s %s\n",
		utils.FormatBalance(distAmount, h.c.Decimals()),
		h.c.Symbol(),
	)
	accounts := make([]*PrivateKey, numAccounts)
	factories := make([]chain.AuthFactory, numAccounts)
	maxPendingMessages := max(pubsub.MaxPendingMessages, txsPerSecond*2)                                                            // ensure we don't block
	dcli, err := rpc.NewWebSocketClient(uris[baseName], rpc.DefaultHandshakeTimeout, maxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	var (
		funds    = map[codec.Address]uint64{}
		fundsL   sync.Mutex
		lastConf int
	)
	for i := 0; i < numAccounts; i++ {
		// Create account
		pk, err := createAccount()
		if err != nil {
			return err
		}
		accounts[i] = pk

		// Create Factory
		f, err := getFactory(pk)
		if err != nil {
			return err
		}
		factories[i] = f

		// Send funds
		_, tx, err := cli.GenerateTransactionManual(parser, nil, getTransfer(pk.Address, distAmount, uniqueBytes()), factory, feePerTx)
		if err != nil {
			return err
		}
		if err := dcli.RegisterTx(tx); err != nil {
			return fmt.Errorf("%w: failed to register tx", err)
		}
		funds[pk.Address] = distAmount

		// Pace the creation of accounts
		if i != 0 && i%1000 == 0 && i != numAccounts {
			if err := confirmTxs(ctx, dcli, 1000); err != nil {
				return err
			}
			utils.Outf("{{yellow}}distributed funds:{{/}} %d/%d accounts\n", i, numAccounts)
			lastConf = i
		}
	}
	// Confirm remaining
	if err := confirmTxs(ctx, dcli, numAccounts-lastConf); err != nil {
		return err
	}
	utils.Outf("{{yellow}}distributed funds:{{/}} %d accounts\n", numAccounts)

	// Kickoff txs
	utils.Outf("{{yellow}}starting load test...{{/}}\n")
	utils.Outf("{{yellow}}target TPS:{{/}} %d\n", txsPerSecond)
	utils.Outf("{{yellow}}Zipf distribution [(v+k)^(-s)] s:{{/}} %.2f {{yellow}}v:{{/}} %.2f\n", sZipf, vZipf)
	clients := []*txIssuer{}
	for i := 0; i < len(uriNames); i++ {
		for j := 0; j < numClients; j++ {
			name := uriNames[i]
			cli := rpc.NewJSONRPCClient(uris[name])
			dcli, err := rpc.NewWebSocketClient(uris[name], rpc.DefaultHandshakeTimeout, maxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				return err
			}
			clients = append(clients, &txIssuer{c: cli, d: dcli, name: name, uri: i})
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
	var pbytes int64
	go func() {
		for {
			select {
			case <-t.C:
				csent := sent.Load()
				cbytes := bytes.Load()
				l.Lock()
				if totalTxs > 0 {
					unitPrices, err = clients[0].c.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% (expired: %.2f%%) {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d (bytes: %.2fKB/s) {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						totalTxs,
						float64(confirmedTxs)/float64(totalTxs)*100,
						float64(expiredTxs)/float64(totalTxs)*100,
						inflight.Load(),
						csent-psent,
						float64(cbytes-pbytes)/units.KiB,
						ParseDimensions(unitPrices),
					)
				}
				l.Unlock()
				psent = csent
				pbytes = cbytes
			case <-cctx.Done():
				return
			}
		}
	}()

	// broadcast txs
	var (
		it = time.NewTimer(0)
		// TODO: make configurable
		z    = rand.NewZipf(rand.New(rand.NewSource(0)), sZipf, vZipf, uint64(numAccounts)-1) //nolint:gosec
		stop bool
	)
	// TODO: log plot of distribution
	for !stop {
		select {
		case <-it.C:
			start := time.Now()
			g := &errgroup.Group{}
			g.SetLimit(maxConcurrency)
			for i := 0; i < txsPerSecond; i++ {
				// Ensure we aren't too backlogged
				if inflight.Load() > int64(maxTxBacklog) {
					break
				}
				g.Go(func() error {
					senderIndex := z.Uint64()
					sender := accounts[senderIndex]
					issuerIndex, issuer := getRandomIssuer(clients)
					factory := factories[senderIndex]
					fundsL.Lock()
					balance := funds[sender.Address]
					if feePerTx > balance {
						fundsL.Unlock()
						return fmt.Errorf("%s has insufficient funds", sender.Address)
					}
					funds[sender.Address] = balance - feePerTx
					fundsL.Unlock()

					// Select tx time
					nextTime := time.Now().Unix()
					tm := &timeModifier{nextTime*consts.MillisecondsPerSecond + parser.Rules(nextTime).GetValidityWindow() - consts.MillisecondsPerSecond /* may be a second early depending on clock sync */}

					// Send transaction
					recipientIndex := z.Uint64()
					if recipientIndex == senderIndex {
						if recipientIndex == uint64(numAccounts-1) {
							recipientIndex--
						} else {
							recipientIndex++
						}
					}
					recipient := accounts[recipientIndex].Address
					action := getTransfer(recipient, 1, uniqueBytes())
					_, tx, err := issuer.c.GenerateTransactionManual(parser, nil, action, factory, feePerTx, tm)
					if err != nil {
						return err
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
							dcli, err := rpc.NewWebSocketClient(uris[issuer.name], rpc.DefaultHandshakeTimeout, maxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
							if err != nil {
								issuer.abandoned = err
								utils.Outf("{{orange}}could not re-create closed issuer:{{/}} %v\n", err)
								issuer.l.Unlock()
								return err
							}
							issuer.d = dcli
							startIssuer(cctx, issuer)
							droppedConfirmations := issuer.outstandingTxs
							issuer.outstandingTxs = 0
							issuer.l.Unlock()
							l.Lock()
							totalTxs += uint64(droppedConfirmations) + 1 // ensure stats are updated
							l.Unlock()
							inflight.Add(-int64(droppedConfirmations))
							utils.Outf("{{green}}re-created closed issuer:{{/}} %d {{yellow}}dropped:{{/}} %d\n", issuerIndex, droppedConfirmations)
						} else {
							// This typically happens when the issuer errors and is replaced by a new one in
							// a different goroutine.
							issuer.l.Unlock()

							// Ensure we track all failures
							l.Lock()
							totalTxs++
							l.Unlock()

							if !errors.Is(err, rpc.ErrClosed) {
								utils.Outf("{{orange}}failed to register tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
							}
						}
						return nil
					}

					issuer.l.Lock()
					issuer.outstandingTxs++
					issuer.l.Unlock()
					inflight.Add(1)
					sent.Add(1)
					bytes.Add(int64(len(tx.Bytes())))
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				exiting.Do(func() {
					utils.Outf("{{yellow}}exiting broadcast loop because of error:{{/}} %v\n", err)
					cancel()
				})
				stop = true
			}

			// Determine how long to sleep
			dur := time.Since(start)
			sleep := max(float64(consts.MillisecondsPerSecond-dur.Milliseconds()), 0)
			it.Reset(time.Duration(sleep) * time.Millisecond)
		case <-cctx.Done():
			stop = true
		case <-signals:
			stop = true
			exiting.Do(func() {
				utils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
				cancel()
			})
		}
	}
	t.Stop()

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
	feePerTx, err = chain.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	utils.Outf("{{yellow}}returning funds (1k batch):{{/}} %s\n", h.c.Address(key.Address))
	var (
		returnedBalance uint64
		returnsSent     int
	)
	lastConf = 0
	for i := 0; i < numAccounts; i++ {
		balance := funds[accounts[i].Address]
		if feePerTx > balance {
			continue
		}
		returnsSent++

		// Send funds
		returnAmt := balance - feePerTx
		f := factories[i]
		_, tx, err := cli.GenerateTransactionManual(parser, nil, getTransfer(key.Address, returnAmt, uniqueBytes()), f, feePerTx)
		if err != nil {
			return err
		}
		if err := dcli.RegisterTx(tx); err != nil {
			return err
		}
		returnedBalance += returnAmt

		// Pace the return of funds
		if i != 0 && i%1000 == 0 && i != numAccounts {
			if err := confirmTxs(ctx, dcli, 1000); err != nil {
				return err
			}
			utils.Outf("{{yellow}}returned funds:{{/}} %d/%d accounts\n", i, numAccounts)
			lastConf = i
		}
	}
	// Confirm remaining
	if err := confirmTxs(ctx, dcli, numAccounts-lastConf); err != nil {
		return err
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
	name           string
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
				issuer.l.Lock()
				// prevents issuer on other thread from handling at the same time
				// needed in case all are stuck
				droppedConfirmations := issuer.outstandingTxs
				issuer.outstandingTxs = 0
				issuer.l.Unlock()
				l.Lock()
				totalTxs += uint64(droppedConfirmations)
				l.Unlock()
				inflight.Add(-int64(droppedConfirmations))
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
				} else {
					expiredTxs++
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

func getRandomIssuer(issuers []*txIssuer) (int, *txIssuer) {
	index := rand.Int() % len(issuers)
	return index, issuers[index]
}

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(issuedTxs.Add(1)))
}

func confirmTxs(ctx context.Context, cli *rpc.WebSocketClient, numTxs int) error {
	for i := 0; i < numTxs; i++ {
		_, dErr, result, err := cli.ListenTx(ctx)
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
	return nil
}
