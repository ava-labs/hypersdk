package loadgen

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

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
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

var (
	maxConcurrency = runtime.NumCPU()
	issuerWg       sync.WaitGroup

	l            sync.Mutex
	confirmedTxs uint64
	totalTxs     uint64

	inflight atomic.Int64
	sent     atomic.Int64
)

type Spammer struct {
	uris    []string
	key     *auth.PrivateKey
	balance uint64

	// Zipf distribution parameters
	zipfSeed *rand.Rand
	sZipf float64
	vZipf float64

	// TPS parameters
	txsPerSecond     int
	minTxsPerSecond  int
	txsPerSecondStep int
	numClients       int // Number of clients per uri node

	// Number of accounts
	numAccounts int
}

func NewSpammer(
	uris []string,
	key *auth.PrivateKey,
	balance uint64,
	sZipf, vZipf float64,
	txsPerSecond, minTxsPerSecond, txsPerSecondStep, numClients, numAccounts int,
) *Spammer {
	// Log Zipf participants
	zipfSeed := rand.New(rand.NewSource(0))

	return &Spammer{
		uris,
		key,
		balance,
		zipfSeed,
		sZipf,
		vZipf,
		txsPerSecond,
		minTxsPerSecond,
		txsPerSecondStep,
		numClients,
		numAccounts,
	}
}

// symbol and decimal used for logging
// TODO: move output to STDOUT into logger
func (s *Spammer) Spam(ctx context.Context, sh SpamHelper, symbol string, decimals uint8) error {
	// log distribution
	s.logZipf(s.zipfSeed)

	// new JSONRPC client
	cli := jsonrpc.NewJSONRPCClient(s.uris[0])

	// TODO: clean up
	factory, err := sh.GetFactory(s.key)
	if err != nil {
		return err
	}

	// Compute max units
	parser, err := sh.GetParser(ctx)
	if err != nil {
		return err
	}
	actions := sh.GetTransfer(s.key.Address, 0, uniqueBytes())
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
	withholding := feePerTx * uint64(s.numAccounts)
	if s.balance < withholding {
		return fmt.Errorf("insufficient funds (have=%d need=%d)", s.balance, withholding)
	}
	distAmount := (s.balance - withholding) / uint64(s.numAccounts)
	utils.Outf(
		"{{yellow}}distributing funds to each account:{{/}} %s %s\n",
		utils.FormatBalance(distAmount, decimals),
		symbol,
	)
	accounts := make([]*auth.PrivateKey, s.numAccounts)
	webSocketClient, err := ws.NewWebSocketClient(s.uris[0], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
	if err != nil {
		return err
	}
	funds := map[codec.Address]uint64{}
	factories := make([]chain.AuthFactory, s.numAccounts)
	var fundsL sync.Mutex
	p := &pacer{ws: webSocketClient}
	go p.Run(ctx, s.minTxsPerSecond)
	for i := 0; i < s.numAccounts; i++ {
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
	utils.Outf("{{yellow}}distributed funds to %d accounts{{/}}\n", s.numAccounts)

	// create issuers
	issuers, err := s.createIssuers(parser)
	if err != nil {
		return err
	}

	// make sure we can exit gracefully & return funds
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
		z = rand.NewZipf(s.zipfSeed, s.sZipf, s.vZipf, uint64(s.numAccounts)-1)

		it                      = time.NewTimer(0)
		currentTarget           = min(s.txsPerSecond, s.minTxsPerSecond)
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
				senderIndex, recipientIndex := z.Uint64(), z.Uint64()
				sender := accounts[senderIndex]
				if recipientIndex == senderIndex {
					if recipientIndex == uint64(s.numAccounts-1) {
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
			if consecutiveUnderBacklog >= successfulRunsToIncreaseTarget && currentTarget < s.txsPerSecond {
				currentTarget = min(currentTarget+s.txsPerSecondStep, s.txsPerSecond)
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
	utils.Outf("{{yellow}}returning funds to %s{{/}}\n", s.key.Address)
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
	go p.Run(ctx, s.minTxsPerSecond)
	time.Sleep(1 * time.Second)
	for i := 0; i < s.numAccounts; i++ {
		// Determine if we should return funds
		balance := funds[accounts[i].Address]
		if feePerTx > balance {
			continue
		}

		// Send funds
		returnAmt := balance - feePerTx
		actions := sh.GetTransfer(s.key.Address, returnAmt, uniqueBytes())
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
		utils.Outf("{{yellow}}returning funds to %s:{{/}} %s %s\n", accounts[i].Address, utils.FormatBalance(returnAmt, decimals), symbol)
	}
	if err := p.Wait(); err != nil {
		utils.Outf("{{orange}}failed to return funds:{{/}} %v\n", err)
		return err
	}
	utils.Outf(
		"{{yellow}}returned funds:{{/}} %s %s\n",
		utils.FormatBalance(returnedBalance, decimals),
		symbol,
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

func uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sent.Add(1)))
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

func (s *Spammer) createIssuers(parser chain.Parser) ([]*issuer, error) {
	issuers := []*issuer{}
	for i := 0; i < len(s.uris); i++ {
		for j := 0; j < s.numClients; j++ {
			cli := jsonrpc.NewJSONRPCClient(s.uris[i])
			webSocketClient, err := ws.NewWebSocketClient(s.uris[i], ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize) // we write the max read
			if err != nil {
				return nil, err
			}
			issuer := &issuer{i: len(issuers), cli: cli, ws: webSocketClient, parser: parser, uri: s.uris[i]}
			issuers = append(issuers, issuer)
		}
	}
	return issuers, nil
}
