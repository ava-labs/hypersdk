// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/loadgen"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	pendingTargetMultiplier        = 10
	successfulRunsToIncreaseTarget = 10
	failedRunsToDecreaseTarget     = 5

	issuerShutdownTimeout = 60 * time.Second
)

var (
	maxConcurrency = runtime.NumCPU()
	issuerWg       sync.WaitGroup

	l            sync.Mutex
	confirmedTxs uint64
	totalTxs     uint64

	inflight atomic.Int64
	sent     atomic.Int64
)

type SpamConfig struct {
	uris    []string
	key     *auth.PrivateKey
	balance uint64

	// Zipf distribution parameters
	sZipf float64
	vZipf float64

	// TPS parameters
	txsPerSecond     int
	minTxsPerSecond  int
	txsPerSecondStep int
	numClients       int

	// Number of accounts
	numAccounts int
}

func (h *Handler) SpamConfigPrompter(sh loadgen.SpamHelper) (*SpamConfig, error) {
	// Select chain
	chains, err := h.GetChains()
	if err != nil {
		return nil, err
	}
	_, uris, err := prompt.SelectChain("select chainID", chains)
	if err != nil {
		return nil, err
	}

	// Select root key
	keys, err := h.GetKeys()
	if err != nil {
		return nil, err
	}
	balances := make([]uint64, len(keys))
	if err := sh.CreateClient(uris[0]); err != nil {
		return nil, err
	}
	for i := 0; i < len(keys); i++ {
		balance, err := sh.LookupBalance(i, keys[i].Address)
		if err != nil {
			return nil, err
		}
		balances[i] = balance
	}

	keyIndex, err := prompt.Choice("select root key", len(keys))
	if err != nil {
		return nil, err
	}
	key := keys[keyIndex]
	balance := balances[keyIndex]

	// Collect parameters
	numAccounts, err := prompt.Int("number of accounts", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	if numAccounts < 2 {
		return nil, ErrInsufficientAccounts
	}
	sZipf, err := prompt.Float("s (Zipf distribution = [(v+k)^(-s)], Default = 1.01)", consts.MaxFloat64)
	if err != nil {
		return nil, err
	}
	vZipf, err := prompt.Float("v (Zipf distribution = [(v+k)^(-s)], Default = 2.7)", consts.MaxFloat64)
	if err != nil {
		return nil, err
	}
		
	txsPerSecond, err := prompt.Int("txs to try and issue per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	minTxsPerSecond, err := prompt.Int("minimum txs to issue per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	txsPerSecondStep, err := prompt.Int("txs to increase per second", consts.MaxInt)
	if err != nil {
		return nil, err
	}
	numClients, err := prompt.Int("number of clients per node", consts.MaxInt)
	if err != nil {
		return nil, err
	}

	return &SpamConfig{
		uris,
		key,
		balance,
		sZipf,
		vZipf,
		txsPerSecond,
		minTxsPerSecond,
		txsPerSecondStep,
		numClients,
		numAccounts,
	}, nil
}

func (h *Handler) Spam(sh loadgen.SpamHelper) error {
	ctx := context.Background()

	config, err := h.SpamConfigPrompter(sh)
	uris := config.uris
	key := config.key
	balance := config.balance
	sZipf := config.sZipf
	vZipf := config.vZipf
	txsPerSecond := config.txsPerSecond
	minTxsPerSecond := config.minTxsPerSecond
	txsPerSecondStep := config.txsPerSecondStep
	numClients := config.numClients
	numAccounts := config.numAccounts
	
	// Log Zipf participants
	zipfSeed := rand.New(rand.NewSource(0))
	config.logZipf(zipfSeed)

	cli := jsonrpc.NewJSONRPCClient(uris[0])
	if err != nil {
		return err
	}

	factory, err := sh.GetFactory(key)
	if err != nil {
		return err
	}

	// No longer using db, so we close
	if err := h.CloseDatabase(); err != nil {
		return err
	}

	// Compute max units
	parser, err := sh.GetParser(ctx)
	if err != nil {
		return err
	}
	actions := sh.GetTransfer(key.Address, 0, uniqueBytes())
	maxUnits, err := chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, factory)
	if err != nil {
		return err
	}

	// Distribute funds
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err := fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	withholding := feePerTx * uint64(numAccounts)
	if balance < withholding {
		return fmt.Errorf("insufficient funds (have=%d need=%d)", balance, withholding)
	}
	distAmount := (balance - withholding) / uint64(numAccounts)
	utils.Outf(
		"{{yellow}}distributing funds to each account:{{/}} %s %s\n",
		utils.FormatBalance(distAmount, h.c.Decimals()),
		h.c.Symbol(),
	)
	accounts := make([]*auth.PrivateKey, numAccounts)
	webSocketClient, err := ws.NewWebSocketClient(uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	funds := map[codec.Address]uint64{}
	factories := make([]chain.AuthFactory, numAccounts)
	var fundsL sync.Mutex
	p := &pacer{ws: webSocketClient}
	go p.Run(ctx, minTxsPerSecond)
	for i := 0; i < numAccounts; i++ {
		// Create account
		pk, err := sh.CreateAccount()
		if err != nil {
			return err
		}
		accounts[i] = pk
		f, err := sh.GetFactory(pk)
		if err != nil {
			return err
		}
		factories[i] = f

		// Send funds
		actions := sh.GetTransfer(pk.Address, distAmount, uniqueBytes())
		_, tx, err := cli.GenerateTransactionManual(parser, actions, factory, feePerTx)
		if err != nil {
			return err
		}
		if err := p.Add(tx); err != nil {
			return fmt.Errorf("%w: failed to register tx", err)
		}
		funds[pk.Address] = distAmount

		// Log progress
		if i%250 == 0 && i > 0 {
			utils.Outf("{{yellow}}issued transfer to %d accounts{{/}}\n", i)
		}
	}
	if err := p.Wait(); err != nil {
		return err
	}
	utils.Outf("{{yellow}}distributed funds to %d accounts{{/}}\n", numAccounts)

	// Kickoff txs
	issuers := []*issuer{}
	for i := 0; i < len(uris); i++ {
		for j := 0; j < numClients; j++ {
			cli := jsonrpc.NewJSONRPCClient(uris[i])
			webSocketClient, err := ws.NewWebSocketClient(uris[i], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				return err
			}
			issuer := &issuer{i: len(issuers), cli: cli, ws: webSocketClient, parser: parser, uri: uris[i]}
			issuers = append(issuers, issuer)
		}
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Start issuers
	unitPrices, err = issuers[0].cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, issuer := range issuers {
		issuer.Start(cctx)
	}

	// Log stats
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
					unitPrices, err = issuers[0].cli.UnitPrices(ctx, false)
					if err != nil {
						continue
					}
					utils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d {{yellow}}unit prices:{{/}} [%s]\n", //nolint:lll
						totalTxs,
						float64(confirmedTxs)/float64(totalTxs)*100,
						inflight.Load(),
						current-psent,
						unitPrices,
					)
				}
				l.Unlock()
				psent = current
			case <-cctx.Done():
				return
			}
		}
	}()

	// Broadcast txs
	var (
		// Do not call this function concurrently (math.Rand is not safe for concurrent use)
		z = rand.NewZipf(zipfSeed, sZipf, vZipf, uint64(numAccounts)-1)

		it                      = time.NewTimer(0)
		currentTarget           = min(txsPerSecond, minTxsPerSecond)
		consecutiveUnderBacklog int
		consecutiveAboveBacklog int

		stop bool
	)
	utils.Outf("{{cyan}}initial target tps:{{/}} %d\n", currentTarget)
	for !stop {
		select {
		case <-it.C:
			start := time.Now()

			// Check to see if we should wait for pending txs
			if int64(currentTarget)+inflight.Load() > int64(currentTarget*pendingTargetMultiplier) {
				consecutiveUnderBacklog = 0
				consecutiveAboveBacklog++
				if consecutiveAboveBacklog >= failedRunsToDecreaseTarget {
					if currentTarget > txsPerSecondStep {
						currentTarget -= txsPerSecondStep
						utils.Outf("{{cyan}}skipping issuance because large backlog detected, decreasing target tps:{{/}} %d\n", currentTarget)
					} else {
						utils.Outf("{{cyan}}skipping issuance because large backlog detected, cannot decrease target{{/}}\n")
					}
					consecutiveAboveBacklog = 0
				}
				it.Reset(1 * time.Second)
				break
			}

			// Issue txs
			g := &errgroup.Group{}
			g.SetLimit(maxConcurrency)
			for i := 0; i < currentTarget; i++ {
				senderIndex, recipientIndex := z.Uint64(), z.Uint64()
				sender := accounts[senderIndex]
				if recipientIndex == senderIndex {
					if recipientIndex == uint64(numAccounts-1) {
						recipientIndex--
					} else {
						recipientIndex++
					}
				}
				recipient := accounts[recipientIndex].Address
				issuer := getRandomIssuer(issuers)
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

					// Send transaction
					actions := sh.GetTransfer(recipient, 1, uniqueBytes())
					return issuer.Send(cctx, actions, factory, feePerTx)
				})
			}

			// Wait for txs to finish
			if err := g.Wait(); err != nil {
				// We don't return here because we want to return funds
				utils.Outf("{{orange}}broadcast loop error:{{/}} %v\n", err)
				stop = true
				break
			}

			// Determine how long to sleep
			dur := time.Since(start)
			sleep := max(float64(consts.MillisecondsPerSecond-dur.Milliseconds()), 0)
			it.Reset(time.Duration(sleep) * time.Millisecond)

			// Check to see if we should increase target
			consecutiveAboveBacklog = 0
			consecutiveUnderBacklog++
			if consecutiveUnderBacklog >= successfulRunsToIncreaseTarget && currentTarget < txsPerSecond {
				currentTarget = min(currentTarget+txsPerSecondStep, txsPerSecond)
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

	// Wait for all issuers to finish
	utils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
	issuerWg.Wait()

	// Return funds
	utils.Outf("{{yellow}}returning funds to %s{{/}}\n", key.Address)
	unitPrices, err = cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err = fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	var returnedBalance uint64
	p = &pacer{ws: webSocketClient}
	go p.Run(ctx, minTxsPerSecond)
	for i := 0; i < numAccounts; i++ {
		// Determine if we should return funds
		balance := funds[accounts[i].Address]
		if feePerTx > balance {
			continue
		}

		// Send funds
		returnAmt := balance - feePerTx
		actions := sh.GetTransfer(key.Address, returnAmt, uniqueBytes())
		_, tx, err := cli.GenerateTransactionManual(parser, actions, factories[i], feePerTx)
		if err != nil {
			return err
		}
		if err := p.Add(tx); err != nil {
			return err
		}
		returnedBalance += returnAmt

		if i%250 == 0 && i > 0 {
			utils.Outf("{{yellow}}checked %d accounts for fund return{{/}}\n", i)
		}
	}
	if err := p.Wait(); err != nil {
		utils.Outf("{{orange}}failed to return funds:{{/}} %v\n", err)
		return err
	}
	utils.Outf(
		"{{yellow}}returned funds:{{/}} %s %s\n",
		utils.FormatBalance(returnedBalance, h.c.Decimals()),
		h.c.Symbol(),
	)
	return nil
}

type pacer struct {
	ws *ws.WebSocketClient

	inflight chan struct{}
	done     chan error
}

func (p *pacer) Run(ctx context.Context, max int) {
	p.inflight = make(chan struct{}, max)
	p.done = make(chan error)

	for range p.inflight {
		_, wsErr, result, err := p.ws.ListenTx(ctx)
		if err != nil {
			p.done <- err
			return
		}
		if wsErr != nil {
			p.done <- wsErr
			return
		}
		if !result.Success {
			// Should never happen
			p.done <- fmt.Errorf("%w: %s", ErrTxFailed, result.Error)
			return
		}
	}
	p.done <- nil
}

func (p *pacer) Add(tx *chain.Transaction) error {
	if err := p.ws.RegisterTx(tx); err != nil {
		return err
	}
	select {
	case p.inflight <- struct{}{}:
		return nil
	case err := <-p.done:
		return err
	}
}

func (p *pacer) Wait() error {
	close(p.inflight)
	return <-p.done
}

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

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sent.Add(1)))
}

func (c *SpamConfig) logZipf(zipfSeed *rand.Rand) {
	zz := rand.NewZipf(zipfSeed, c.sZipf, c.vZipf, uint64(c.numAccounts)-1)
	trials := c.txsPerSecond * 60 * 2 // sender/receiver
	unique := set.NewSet[uint64](trials)
	for i := 0; i < trials; i++ {
		unique.Add(zz.Uint64())
	}
	utils.Outf("{{blue}}unique participants expected every 60s:{{/}} %d\n", unique.Len())
}