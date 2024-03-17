// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package cli

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
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
	"github.com/ava-labs/hypersdk/window"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

const (
	plotSamples    = 1000
	plotIdentities = 10
	plotBarWidth   = 10
	plotOverhead   = 10
	plotHeight     = 25

	pendingTargetMultiplier        = 5
	pendingExpiryBuffer            = 15 * consts.MillisecondsPerSecond
	targetIncreaseRate             = 1000
	successfulRunsToIncreaseTarget = 10
	failedRunsToDecreaseTarget     = 5

	issuerShutdownTimeout = 60 * time.Second
)

type txWrapper struct {
	id       ids.ID
	expiry   int64
	issuance int64
}

func (w *txWrapper) ID() ids.ID {
	return w.id
}

func (w *txWrapper) Expiry() int64 {
	return w.expiry
}

// TODO: we should NEVER use globals, remove this
var (
	ctx            = context.Background()
	maxConcurrency = runtime.NumCPU()

	issuerWg sync.WaitGroup

	l                sync.Mutex
	confirmationTime uint64 // reset each second
	delayTime        int64  // reset each second
	delayCount       uint64 // reset each second
	successTxs       uint64
	expiredTxs       uint64
	failedTxs        uint64
	invalidTxs       uint64
	totalTxs         uint64

	pending = NewOExpirer()

	issuedTxs atomic.Int64
	sent      atomic.Int64
	bytes     atomic.Int64
)

func (h *Handler) Spam(
	numAccounts int, txsPerSecond int,
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
	rules := parser.Rules(time.Now().UnixMilli())
	maxUnits, err := chain.EstimateUnits(rules, action, factory, nil)
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
	maxPendingMessages := max(pubsub.MaxPendingMessages, txsPerSecond*2) // ensure we don't block
	dcli, err := rpc.NewWebSocketClient(
		uris[baseName],
		rpc.DefaultHandshakeTimeout,
		maxPendingMessages,
		pubsub.TargetWriteMessageSize,
		pubsub.MaxReadMessageSize,
	) // we write the max read
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
	utils.Outf("{{yellow}}max tps:{{/}} %d\n", txsPerSecond)
	utils.Outf("{{yellow}}zipf distribution [(v+k)^(-s)] s:{{/}} %.2f {{yellow}}v:{{/}} %.2f\n", sZipf, vZipf)
	clients := []*txIssuer{}
	for i := 0; i < len(uriNames); i++ {
		for j := 0; j < numClients; j++ {
			name := uriNames[i]
			cli := rpc.NewJSONRPCClient(uris[name])
			dcli, err := rpc.NewWebSocketClient(
				uris[name],
				rpc.DefaultHandshakeTimeout,
				maxPendingMessages,
				pubsub.TargetWriteMessageSize,
				pubsub.MaxReadMessageSize,
			) // we write the max read
			if err != nil {
				return err
			}
			clients = append(clients, &txIssuer{c: cli, d: dcli, name: name, uri: i})
			if len(clients)%25 == 0 && len(clients) > 0 {
				utils.Outf("{{yellow}}initializing connections:{{/}} %d/%d clients\n", len(clients), len(uriNames)*numClients)
			}
		}
	}
	utils.Outf("{{yellow}}initialized connections:{{/}} %d clients\n", len(clients))
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// confirm txs (track failure rate)
	unitPrices, err = clients[0].c.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, client := range clients {
		startConfirmer(cctx, client.d)
	}

	// log stats
	t := time.NewTicker(1 * time.Second) // ensure no duplicates created
	defer t.Stop()
	var (
		lastExpiredCount uint64
		lastOnchainCount uint64
		psent            int64
		pbytes           int64

		startRun      = time.Now()
		tpsWindow     = window.Window{}
		expiredWindow = window.Window{} // needed for ttfWindow but not tps
		ttfWindow     = window.Window{} // max int64 is ~5.3M (so can't have more than that per second)
	)
	go func() {
		for {
			select {
			case <-t.C:
				csent := sent.Load()
				cbytes := bytes.Load()
				l.Lock()
				now := time.Now().UnixMilli()
				dropped := pending.SetMin(now - pendingExpiryBuffer) // set in the past to allow for delay on connection
				for _, txw := range dropped {
					confirmationTime += uint64(now - txw.issuance - pendingExpiryBuffer)
					expiredTxs++
					totalTxs++
				}
				cConf := confirmationTime
				confirmationTime = 0
				newTTFWindow, err := window.Roll(ttfWindow, 1)
				if err != nil {
					panic(err)
				}
				ttfWindow = newTTFWindow
				window.Update(&ttfWindow, window.WindowSliceSize-consts.Uint64Len, cConf)

				onchainTxs := successTxs + failedTxs
				cTxs := onchainTxs - lastOnchainCount
				lastOnchainCount = onchainTxs
				newTPSWindow, err := window.Roll(tpsWindow, 1)
				if err != nil {
					panic(err)
				}
				tpsWindow = newTPSWindow
				window.Update(&tpsWindow, window.WindowSliceSize-consts.Uint64Len, cTxs)
				tpsDivisor := min(window.WindowSize, time.Since(startRun).Seconds())
				tpsSum := window.Sum(tpsWindow)

				cExpired := expiredTxs - lastExpiredCount
				lastExpiredCount = expiredTxs
				newExpiredWindow, err := window.Roll(expiredWindow, 1)
				if err != nil {
					panic(err)
				}
				expiredWindow = newExpiredWindow
				window.Update(&expiredWindow, window.WindowSliceSize-consts.Uint64Len, cExpired)

				cDelayTime := delayTime
				delayTime = 0
				cDelayCount := delayCount
				delayCount = 0

				if totalTxs > 0 && tpsSum > 0 {
					// tps is only contains transactions that actually made it onchain
					// ttf includes all transactions that made it onchain or expired, but not transactions that returned an error on submission
					utils.Outf(
						"{{yellow}}tps:{{/}} %.2f {{yellow}}ttf:{{/}} %.2fs {{yellow}}total txs:{{/}} %d (invalid=%.2f%% failed=%.2f%% expired=%.2f%%) {{yellow}}issued/s:{{/}} %d (pending=%d) {{yellow}}outbound bandwidth/s:{{/}} %.2fKB {{yellow}}msgs recv/s:{{/}} %d (recv delay=%.2fms)\n", //nolint:lll
						float64(tpsSum)/float64(tpsDivisor),
						float64(window.Sum(ttfWindow))/float64(tpsSum+window.Sum(expiredWindow))/float64(consts.MillisecondsPerSecond),
						totalTxs,
						float64(invalidTxs)/float64(totalTxs)*100,
						float64(failedTxs)/float64(totalTxs)*100,
						float64(expiredTxs)/float64(totalTxs)*100,
						csent-psent,
						pending.Len(),
						float64(cbytes-pbytes)/units.KiB,
						cDelayCount,
						float64(cDelayTime)/float64(cDelayCount),
					)
				} else if totalTxs > 0 {
					// This shouldn't happen but we should log when it does.
					utils.Outf(
						"{{yellow}}total txs:{{/}} %d (pre-errored=%.2f%% errored=%.2f%% expired=%.2f%%) {{yellow}}issued/s:{{/}} %d (pending=%d) {{yellow}}bandwidth/s:{{/}} %.2fKB\n", //nolint:lll
						totalTxs,
						float64(invalidTxs)/float64(totalTxs)*100,
						float64(failedTxs)/float64(totalTxs)*100,
						float64(expiredTxs)/float64(totalTxs)*100,
						csent-psent,
						pending.Len(),
						float64(cbytes-pbytes)/units.KiB,
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
		z = rand.NewZipf(rand.New(rand.NewSource(0)), sZipf, vZipf, uint64(numAccounts)-1)

		it                      = time.NewTimer(0)
		currentTarget           = min(txsPerSecond, targetIncreaseRate)
		consecutiveUnderBacklog int
		consecutiveAboveBacklog int

		stop bool
	)
	utils.Outf("{{cyan}}initial target tps:{{/}} %d\n", currentTarget)
	for !stop {
		select {
		case <-it.C:
			start := time.Now()
			g := &errgroup.Group{}
			g.SetLimit(maxConcurrency)
			exitedEarly := false
			for i := 0; i < currentTarget; i++ {
				// Ensure we aren't too backlogged
				if pending.Len() > currentTarget*pendingTargetMultiplier {
					consecutiveAboveBacklog++
					if consecutiveAboveBacklog == failedRunsToDecreaseTarget {
						if currentTarget > targetIncreaseRate {
							currentTarget -= targetIncreaseRate
							utils.Outf("{{cyan}}skipping issuance because large backlog detected, decreasing target tps:{{/}} %d\n", currentTarget)
						} else {
							utils.Outf("{{cyan}}skipping issuance because large backlog detected, cannot decrease target{{/}}\n")
						}
						consecutiveAboveBacklog = 0
					} else {
						utils.Outf("{{cyan}}skipping issuance because large backlog detected{{/}}\n")
					}
					exitedEarly = true
					break
				}
				// math.Rand is not safe for concurrent use
				senderIndex := z.Uint64()
				sender := accounts[senderIndex]
				recipientIndex := z.Uint64()
				issuerIndex, issuer := getRandomIssuer(clients)
				g.Go(func() error {
					factory := factories[senderIndex]
					fundsL.Lock()
					balance := funds[sender.Address]
					if feePerTx > balance {
						fundsL.Unlock()
						utils.Outf("{{orange}}tx has insufficient funds:{{/}} %s\n", sender.Address)
						return fmt.Errorf("%s has insufficient funds", sender.Address)
					}
					funds[sender.Address] = balance - feePerTx
					fundsL.Unlock()

					// Select tx time
					nextTime := time.Now().Unix()
					tm := &timeModifier{nextTime*consts.MillisecondsPerSecond + parser.Rules(nextTime).GetValidityWindow() - consts.MillisecondsPerSecond /* may be a second early depending on clock sync */}

					// Send transaction
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
						utils.Outf("{{orange}}failed to generate tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
						return err
					}
					pending.Add(&txWrapper{id: tx.ID(), expiry: tx.Expiry(), issuance: time.Now().UnixMilli()})
					if err := issuer.d.RegisterTx(tx); err != nil {
						issuer.l.Lock()
						if issuer.d.Closed() {
							if issuer.abandoned != nil {
								issuer.l.Unlock()
								utils.Outf("{{orange}}issuer abandoned:{{/}} %v\n", issuer.abandoned)
								return issuer.abandoned
							}
							// recreate issuer
							utils.Outf("{{orange}}re-creating issuer:{{/}} %d {{orange}}uri:{{/}} %d {{red}}err:{{/}} %v\n", issuerIndex, issuer.uri, err)
							dcli, err := rpc.NewWebSocketClient(
								uris[issuer.name],
								rpc.DefaultHandshakeTimeout,
								maxPendingMessages,
								pubsub.TargetWriteMessageSize,
								pubsub.MaxReadMessageSize,
							) // we write the max read
							if err != nil {
								issuer.abandoned = err
								issuer.l.Unlock()
								utils.Outf("{{orange}}could not re-create closed issuer:{{/}} %v\n", err)
								return err
							}
							issuer.d = dcli
							issuer.l.Unlock()
							utils.Outf("{{green}}re-created closed issuer:{{/}} %d (%v)\n", issuerIndex, err)
							startConfirmer(cctx, dcli)
						} else {
							// This typically happens when the issuer errors and is replaced by a new one in
							// a different goroutine.
							issuer.l.Unlock()
							utils.Outf("{{orange}}failed to register tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
						}
						return nil
					}
					sent.Add(1)
					bytes.Add(int64(len(tx.Bytes())))
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				utils.Outf("{{yellow}}exiting broadcast loop because of error:{{/}} %v\n", err)
				cancel()
				stop = true
			}

			// Determine how long to sleep
			dur := time.Since(start)
			sleep := max(float64(consts.MillisecondsPerSecond-dur.Milliseconds()), 0)
			it.Reset(time.Duration(sleep) * time.Millisecond)

			// Determine next target
			if exitedEarly {
				consecutiveUnderBacklog = 0
				continue
			}
			consecutiveAboveBacklog = 0
			consecutiveUnderBacklog++
			if consecutiveUnderBacklog == successfulRunsToIncreaseTarget && currentTarget < txsPerSecond {
				currentTarget = min(currentTarget+targetIncreaseRate, txsPerSecond)
				utils.Outf("{{cyan}}increasing target tps:{{/}} %d\n", currentTarget)
				consecutiveUnderBacklog = 0
			}
		case <-cctx.Done():
			stop = true
			utils.Outf("{{yellow}}context canceled{{/}}\n")
		case <-signals:
			stop = true
			utils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
			cancel()
		}
	}
	it.Stop()

	// Wait for all issuers to finish
	utils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
	dctx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				pending.SetMin(time.Now().UnixMilli())
				utils.Outf("{{yellow}}remaining:{{/}} %d\n", pending.Len())
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
		"{{yellow}}returned funds from %d accounts:{{/}} %s %s\n",
		numAccounts,
		utils.FormatBalance(returnedBalance, h.c.Decimals()),
		h.c.Symbol(),
	)
	return nil
}

type txIssuer struct {
	c *rpc.JSONRPCClient
	d *rpc.WebSocketClient

	l         sync.Mutex
	name      string
	uri       int
	abandoned error
}

type timeModifier struct {
	Timestamp int64
}

func (t *timeModifier) Base(b *chain.Base) {
	b.Timestamp = t.Timestamp
}

func startConfirmer(cctx context.Context, c *rpc.WebSocketClient) {
	issuerWg.Add(1)
	go func() {
		for {
			txID, status, err := c.ListenTx(context.TODO())
			if err != nil {
				utils.Outf("{{red}}unable to listen for tx{{/}}: %v\n", err)
				return
			}
			now := time.Now().UnixMilli()
			tw := pending.Remove(txID)
			if tw == nil {
				// This could happen if we've removed the transaction from pending after [pendingExpiryBuffer].
				// This will be counted by the loop that did that, so we can just continue.
				continue
			}
			l.Lock()
			switch status {
			case rpc.TxSuccess:
				successTxs++
			case rpc.TxFailed:
				failedTxs++
			case rpc.TxExpired:
				expiredTxs++
			case rpc.TxInvalid:
				invalidTxs++
			default:
				utils.Outf("{{red}}unknown tx status{{/}}: %d\n", status)
				return
			}
			if status <= rpc.TxExpired {
				confirmationTime += uint64(now - tw.issuance)
			}
			totalTxs++
			l.Unlock()
		}
	}()
	ct := time.NewTicker(time.Second)
	go func() {
		defer ct.Stop()

		for {
			select {
			case <-ct.C:
				if c.Closed() {
					return
				}
				dt, dc := c.ResetDelay()
				l.Lock()
				delayTime += dt
				delayCount += dc
				l.Unlock()
			case <-cctx.Done():
				return
			}
		}
	}()
	go func() {
		defer func() {
			_ = c.Close()
			issuerWg.Done()
		}()

		<-cctx.Done()
		start := time.Now()
		for time.Since(start) < issuerShutdownTimeout {
			if c.Closed() {
				return
			}
			if pending.Len() == 0 {
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
		_, status, err := cli.ListenTx(ctx)
		if err != nil {
			return err
		}
		if status != rpc.TxSuccess {
			// Should never happen
			return fmt.Errorf("transaction failed: %d", status)
		}
	}
	return nil
}
