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
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
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

type SpamHelper interface {
	CreateAccount() (*PrivateKey, error)
	GetFactory(pk *PrivateKey) (chain.AuthFactory, error)

	CreateClient(uri string, networkID uint32, chainID ids.ID) error
	GetParser(ctx context.Context, chainID ids.ID) (chain.Parser, error)
	LookupBalance(choice int, address string) (uint64, error)

	GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action
}

// TODO: make these functions an interface rather than functions
func (h *Handler) Spam(sh SpamHelper) error {
	ctx := context.Background()

	// Select chain
	chainID, uris, err := h.PromptChain("select chainID", nil)
	if err != nil {
		return err
	}
	cli := rpc.NewJSONRPCClient(uris[0])
	networkID, _, _, err := cli.Network(ctx)
	if err != nil {
		return err
	}

	// Select root key
	keys, err := h.GetKeys()
	if err != nil {
		return err
	}
	balances := make([]uint64, len(keys))
	if err := sh.CreateClient(uris[0], networkID, chainID); err != nil {
		return err
	}
	for i := 0; i < len(keys); i++ {
		address := h.c.Address(keys[i].Address)
		balance, err := sh.LookupBalance(i, address)
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
	factory, err := sh.GetFactory(key)
	if err != nil {
		return err
	}

	// No longer using db, so we close
	if err := h.CloseDatabase(); err != nil {
		return err
	}

	// Compute max units
	parser, err := sh.GetParser(ctx, chainID)
	if err != nil {
		return err
	}
	actions := sh.GetTransfer(keys[0].Address, 0, uniqueBytes())
	maxUnits, err := chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, factory)
	if err != nil {
		return err
	}

	// Collect parameters
	numAccounts, err := h.PromptInt("number of accounts", consts.MaxInt)
	if err != nil {
		return err
	}
	if numAccounts < 2 {
		return fmt.Errorf("must have at least 2 accounts")
	}
	sZipf, err := h.PromptFloat("s (Zipf distribution = [(v+k)^(-s)], Default = 1.01)", consts.MaxFloat64)
	if err != nil {
		return err
	}
	vZipf, err := h.PromptFloat("v (Zipf distribution = [(v+k)^(-s)], Default = 2.7)", consts.MaxFloat64)
	if err != nil {
		return err
	}
	txsPerSecond, err := h.PromptInt("txs to try and issue per second", consts.MaxInt)
	if err != nil {
		return err
	}
	minTxsPerSecond, err := h.PromptInt("minimum txs to issue per second", consts.MaxInt)
	if err != nil {
		return err
	}
	txsPerSecondStep, err := h.PromptInt("txs to increase per second", consts.MaxInt)
	if err != nil {
		return err
	}
	numClients, err := h.PromptInt("number of clients per node", consts.MaxInt)
	if err != nil {
		return err
	}

	// Log Zipf participants
	zipfSeed := rand.New(rand.NewSource(0))
	zz := rand.NewZipf(zipfSeed, sZipf, vZipf, uint64(numAccounts)-1)
	trials := txsPerSecond * 60 * 2 // sender/receiver
	unique := set.NewSet[uint64](trials)
	for i := 0; i < trials; i++ {
		unique.Add(zz.Uint64())
	}
	utils.Outf("{{blue}}unique participants expected every 60s:{{/}} %d\n", unique.Len())

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
	accounts := make([]*PrivateKey, numAccounts)
	dcli, err := rpc.NewWebSocketClient(uris[0], rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	funds := map[codec.Address]uint64{}
	factories := make([]chain.AuthFactory, numAccounts)
	var fundsL sync.Mutex
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
		_, tx, err := cli.GenerateTransactionManual(parser, sh.GetTransfer(pk.Address, distAmount, nil), factory, feePerTx)
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
			return fmt.Errorf("%w: %s", ErrTxFailed, result.Error)
		}
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

					// Send transaction
					actions := sh.GetTransfer(recipient, 1, uniqueBytes())
					_, tx, err := issuer.c.GenerateTransactionManual(parser, actions, factory, feePerTx)
					if err != nil {
						utils.Outf("{{orange}}failed to generate tx:{{/}} %v\n", err)
						return fmt.Errorf("failed to generate tx: %w", err)
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

						// If issuance fails during retry, we should fail
						return issuer.d.RegisterTx(tx)
					}
					return nil
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
	unitPrices, err = cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err = fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	utils.Outf("{{yellow}}returning funds to %s{{/}}\n", h.c.Address(key.Address))
	var (
		returnedBalance uint64
		returns         = make(chan struct{}, txsPerSecondStep)
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
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
			if err != nil {
				return err
			}
			if err := dcli.RegisterTx(tx); err != nil {
				return err
			}
			returnedBalance += returnAmt
			returns <- struct{}{}
		}
		close(returns)
		return nil
	})
	g.Go(func() error {
		for range returns {
			_, dErr, result, err := dcli.ListenTx(gctx)
			if err != nil {
				return err
			}
			if dErr != nil {
				return dErr
			}
			if !result.Success {
				// Should never happen
				return fmt.Errorf("%w: %s", ErrTxFailed, result.Error)
			}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
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

type txIssuer struct {
	c *rpc.JSONRPCClient
	d *rpc.WebSocketClient

	l              sync.Mutex
	uri            int
	outstandingTxs int

	abandoned error
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
					utils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Error), result.Success)
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

func getRandomIssuer(issuers []*txIssuer) (int, *txIssuer) {
	index := rand.Int() % len(issuers)
	return index, issuers[index]
}

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sent.Add(1)))
}
