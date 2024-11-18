// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/utils/set"
	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	pendingTargetMultiplier        = 10
	successfulRunsToIncreaseTarget = 10
	failedRunsToDecreaseTarget     = 5

	issuerShutdownTimeout = 60 * time.Second
)

// TODO: remove the use of global variables
var (
	maxConcurrency = runtime.NumCPU()
)

type Spammer struct {
	uris    []string
	key     *auth.PrivateKey
	balance uint64

	// Zipf distribution parameters
	zipfSeed *rand.Rand
	sZipf    float64
	vZipf    float64

	// TPS parameters
	txsPerSecond     int
	minTxsPerSecond  int
	txsPerSecondStep int
	numClients       int // Number of clients per uri node

	// Number of accounts
	numAccounts int

	// keep track of variables shared across issuers
	tracker *tracker
}

func NewSpammer(sc *Config, sh SpamHelper) (*Spammer, error) {
	// Log Zipf participants
	zipfSeed := rand.New(rand.NewSource(0)) //nolint:gosec
	tracker := &tracker{}
	balance, err := sh.LookupBalance(sc.key.Address)
	if err != nil {
		return nil, err
	}

	return &Spammer{
		uris:     sc.uris,
		key:      sc.key,
		balance:  balance,
		zipfSeed: zipfSeed,
		sZipf:    sc.sZipf,
		vZipf:    sc.vZipf,

		txsPerSecond:     sc.txsPerSecond,
		minTxsPerSecond:  sc.minTxsPerSecond,
		txsPerSecondStep: sc.txsPerSecondStep,
		numClients:       sc.numClients,
		numAccounts:      sc.numAccounts,

		tracker: tracker,
	}, nil
}

// Spam tests the throughput of the network by sending transactions using
// multiple accounts and clients. It first distributes funds to the accounts
// and then sends transactions between the accounts. It returns the funds to
// the original account after the test is complete.
// [sh] injects the necessary functions to interact with the network.
// [terminate] if true, the spammer will stop after reaching the target TPS.
// [symbol] and [decimals] are used to format the output.
func (s *Spammer) Spam(ctx context.Context, sh SpamHelper, terminate bool, symbol string) error {
	// log distribution
	s.logZipf(s.zipfSeed)

	// new JSONRPC client
	cli := jsonrpc.NewJSONRPCClient(s.uris[0])

	factory, err := auth.GetFactory(s.key)
	if err != nil {
		return err
	}

	// Compute max units
	parser, err := sh.GetParser(ctx)
	if err != nil {
		return err
	}
	actions := sh.GetTransfer(s.key.Address, 0, s.tracker.uniqueBytes())
	maxUnits, err := chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, factory)
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

	// distribute funds
	accounts, factories, err := s.distributeFunds(ctx, cli, parser, feePerTx, sh)
	if err != nil {
		return err
	}

	// create issuers
	issuers, err := s.createIssuers(parser)
	if err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, issuer := range issuers {
		issuer.Start(cctx)
	}

	// set logging
	s.tracker.logState(cctx, issuers[0].cli)

	// broadcast transactions
	err = s.broadcast(cctx, sh, accounts, factories, issuers, feePerTx, terminate)
	cancel()
	if err != nil {
		return err
	}

	// Wait for all issuers to finish
	utils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
	s.tracker.issuerWg.Wait()

	maxUnits, err = chain.EstimateUnits(parser.Rules(time.Now().UnixMilli()), actions, factory)
	if err != nil {
		return err
	}
	return s.returnFunds(ctx, cli, parser, maxUnits, sh, accounts, factories, symbol)
}

func (s Spammer) broadcast(
	ctx context.Context,
	sh SpamHelper,
	accounts []*auth.PrivateKey,

	factories []chain.AuthFactory,
	issuers []*issuer,

	feePerTx uint64,
	terminate bool,
) error {
	// make sure we can exit gracefully & return funds
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	var (
		// Do not call this function concurrently (math.Rand is not safe for concurrent use)
		z = rand.NewZipf(s.zipfSeed, s.sZipf, s.vZipf, uint64(s.numAccounts)-1)

		it                      = time.NewTimer(0)
		currentTarget           = min(s.txsPerSecond, s.minTxsPerSecond)
		consecutiveUnderBacklog int
		consecutiveAboveBacklog int
		broadcastErr            error
		stop                    bool
	)
	utils.Outf("{{cyan}}initial target tps:{{/}} %d\n", currentTarget)
	for !stop {
		select {
		case <-it.C:
			start := time.Now()

			// Check to see if we should wait for pending txs
			if int64(currentTarget)+s.tracker.inflight.Load() > int64(currentTarget*pendingTargetMultiplier) {
				consecutiveUnderBacklog = 0
				consecutiveAboveBacklog++
				if consecutiveAboveBacklog >= failedRunsToDecreaseTarget {
					if currentTarget > s.txsPerSecondStep {
						currentTarget -= s.txsPerSecondStep
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
				senderIndex := z.Uint64()
				sender := accounts[senderIndex].Address
				issuer := getRandomIssuer(issuers)
				g.Go(func() error {
					factory := factories[senderIndex]
					balance, err := sh.LookupBalance(sender)
					if err != nil {
						return err
					}
					if balance < feePerTx {
						return fmt.Errorf("insufficient funds (have=%d need=%d)", balance, feePerTx)
					}
					// Send transaction
					actions := sh.GetActions()
					return issuer.Send(ctx, actions, factory, feePerTx)
				})
			}

			// Wait for txs to finish
			if err := g.Wait(); err != nil {
				// We don't return here because we want to return funds
				utils.Outf("{{orange}}broadcast loop error:{{/}} %v\n", err)
				broadcastErr = err
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
			// once desired TPS is reached, stop the spammer
			if terminate && currentTarget == s.txsPerSecond && consecutiveUnderBacklog >= successfulRunsToIncreaseTarget {
				utils.Outf("{{green}}reached target tps:{{/}} %d\n", currentTarget)
				stop = true
			} else if consecutiveUnderBacklog >= successfulRunsToIncreaseTarget && currentTarget < s.txsPerSecond {
				currentTarget = min(currentTarget+s.txsPerSecondStep, s.txsPerSecond)
				utils.Outf("{{cyan}}increasing target tps:{{/}} %d\n", currentTarget)
				consecutiveUnderBacklog = 0
			}
		case <-ctx.Done():
			stop = true
			utils.Outf("{{yellow}}context canceled{{/}}\n")
		case <-signals:
			stop = true
			utils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
		}
	}

	return broadcastErr
}

func (s *Spammer) logZipf(zipfSeed *rand.Rand) {
	zz := rand.NewZipf(zipfSeed, s.sZipf, s.vZipf, uint64(s.numAccounts)-1)
	trials := s.txsPerSecond * 60 * 2 // sender/receiver
	unique := set.NewSet[uint64](trials)
	for i := 0; i < trials; i++ {
		unique.Add(zz.Uint64())
	}
	utils.Outf("{{blue}}unique participants expected every 60s:{{/}} %d\n", unique.Len())
}

// createIssuer creates an [numClients] transaction issuers for each URI in [uris]
func (s *Spammer) createIssuers(parser chain.Parser) ([]*issuer, error) {
	issuers := []*issuer{}

	for i := 0; i < len(s.uris); i++ {
		for j := 0; j < s.numClients; j++ {
			cli := jsonrpc.NewJSONRPCClient(s.uris[i])
			webSocketClient, err := ws.NewWebSocketClient(s.uris[i], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				return nil, err
			}
			issuer := &issuer{
				i:       len(issuers),
				cli:     cli,
				ws:      webSocketClient,
				parser:  parser,
				uri:     s.uris[i],
				tracker: s.tracker,
			}
			issuers = append(issuers, issuer)
		}
	}
	return issuers, nil
}

func (s *Spammer) distributeFunds(ctx context.Context, cli *jsonrpc.JSONRPCClient, parser chain.Parser, feePerTx uint64, sh SpamHelper) ([]*auth.PrivateKey, []chain.AuthFactory, error) {
	withholding := feePerTx * uint64(s.numAccounts)
	if s.balance < withholding {
		return nil, nil, fmt.Errorf("insufficient funds (have=%d need=%d)", s.balance, withholding)
	}

	distAmount := (s.balance - withholding) / uint64(s.numAccounts*5)

	utils.Outf("{{yellow}}distributing funds to each account{{/}}\n")

	accounts := make([]*auth.PrivateKey, s.numAccounts)
	factories := make([]chain.AuthFactory, s.numAccounts)

	factory, err := auth.GetFactory(s.key)
	if err != nil {
		return nil, nil, err
	}

	webSocketClient, err := ws.NewWebSocketClient(s.uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return nil, nil, err
	}
	p := &pacer{ws: webSocketClient}
	go p.Run(ctx, s.minTxsPerSecond)
	// TODO: we sleep here because occasionally the pacer will hang. Potentially due to
	// p.wait() closing the inflight channel before the tx is registered/sent. Debug more.
	time.Sleep(3 * time.Second)
	for i := 0; i < s.numAccounts; i++ {
		// Create account
		pk, err := sh.CreateAccount()
		if err != nil {
			return nil, nil, err
		}
		accounts[i] = pk
		f, err := auth.GetFactory(pk)
		if err != nil {
			return nil, nil, err
		}
		factories[i] = f

		// Send funds
		actions := sh.GetTransfer(pk.Address, distAmount, s.tracker.uniqueBytes())
		_, tx, err := cli.GenerateTransactionManual(parser, actions, factory, feePerTx)
		if err != nil {
			return nil, nil, err
		}
		if err := p.Add(tx); err != nil {
			return nil, nil, fmt.Errorf("%w: failed to register tx", err)
		}

		// Log progress
		if i%250 == 0 && i > 0 {
			utils.Outf("{{yellow}}issued transfer to %d accounts{{/}}\n", i)
		}
	}
	if err := p.Wait(); err != nil {
		return nil, nil, err
	}
	utils.Outf("{{yellow}}distributed funds to %d accounts{{/}}\n", s.numAccounts)

	return accounts, factories, nil
}

func (s *Spammer) returnFunds(ctx context.Context, cli *jsonrpc.JSONRPCClient, parser chain.Parser, maxUnits fees.Dimensions, sh SpamHelper, accounts []*auth.PrivateKey, factories []chain.AuthFactory, symbol string) error {
	// Return funds
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err := fees.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	utils.Outf("{{yellow}}returning funds to %s{{/}}\n", s.key.Address)
	var returnedBalance uint64

	webSocketClient, err := ws.NewWebSocketClient(s.uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	p := &pacer{ws: webSocketClient}
	go p.Run(ctx, s.minTxsPerSecond)
	// TODO: we sleep here because occasionally the pacer will hang. Potentially due to
	// p.wait() closing the inflight channel before the tx is registered/sent. Debug more.
	time.Sleep(3 * time.Second)
	for i := 0; i < s.numAccounts; i++ {
		// Determine if we should return funds
		balance, err := sh.LookupBalance(accounts[i].Address)
		if err != nil {
			return err
		}
		if feePerTx > balance {
			continue
		}

		// Send funds
		returnAmt := balance - feePerTx
		actions := sh.GetTransfer(s.key.Address, returnAmt, s.tracker.uniqueBytes())
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
		utils.Outf("{{yellow}}returning funds to %s:{{/}} %s %s\n", accounts[i].Address, utils.FormatBalance(returnAmt), symbol)
	}
	if err := p.Wait(); err != nil {
		utils.Outf("{{orange}}failed to return funds:{{/}} %v\n", err)
		return err
	}
	utils.Outf(
		"{{yellow}}returned funds:{{/}} %s %s\n",
		utils.FormatBalance(returnedBalance),
		symbol,
	)
	return nil
}
