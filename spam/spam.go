package spam

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
)


type Spammer struct {
	// handler kept from the CLI
	H *cli.Handler

	// number of accounts to distribute funds to
	NumAccounts int
	// number of transactions to send per second
	TxsPerSecond int
	// minimum amount of TPS to start with
	MinCapacity int
	// amount to increase TPS by each step
	StepSize int

	// distribution 
	Zipf *ZipfDistribution

	NumClients int
	ClusterInfo string
	Hrp string
	PrivateKey *cli.PrivateKey
}

func (s *Spammer) Spam(
	createClient func(string, uint32, ids.ID) error, // must save on caller side
	getFactory func(*cli.PrivateKey) (chain.AuthFactory, error),
	createAccount func() (*cli.PrivateKey, error),
	lookupBalance func(int, string) (uint64, error),
	getParser func(context.Context, ids.ID) (chain.Parser, error),
	getTransfer func(codec.Address, bool, uint64, []byte) chain.Action, // []byte prevents duplicate txs
	submitDummy func(*rpc.JSONRPCClient, *cli.PrivateKey) func(context.Context, uint64) error,
) error {
	// Plot Zipf
	s.Zipf.Plot()
	s.Zipf.ExpectedParticipants(s.NumAccounts, s.TxsPerSecond)
	
	// validate inputs
	err := s.ValidateInputs()
	if err != nil {
		return err
	}

	// Select chain
	if len(s.ClusterInfo) == 0 {
		chainID, uris, err = s.H.PromptChain("select chainID", nil)
	} else {
		chainID, uris, err = cli.ReadCLIFile(s.ClusterInfo)
	}
	if err != nil {
		return err
	}
	uriNames := onlyAPIs(uris)
	baseName := uriNames[0]

	// Select root key
	client := rpc.NewJSONRPCClient(uris[baseName])
	networkID, _, _, err := client.Network(ctx)
	if err != nil {
		return err
	}
	if err := createClient(uris[baseName], networkID, chainID); err != nil {
		return err
	}
	
	// initialize root key and balance
	key, balance, err := s.InitializeRootKey(lookupBalance)
	if err != nil {
		return err
	}

	factory, err := getFactory(key)
	if err != nil {
		return err
	}

	// Compute max units
	parser, err = getParser(ctx, chainID)
	if err != nil {
		return err
	}
	action := getTransfer(key.Address, true, 0, uniqueBytes())
	rules := parser.Rules(time.Now().UnixMilli())
	maxUnits, err := chain.EstimateUnits(rules, action, factory, nil)
	if err != nil {
		return err
	}
	validityWindow = rules.GetValidityWindow()

	unitPrices, err := client.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err := chain.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	witholding := feePerTx * uint64(s.NumAccounts)
	if balance < witholding {
		return fmt.Errorf("insufficient funds (have=%d need=%d)", balance, witholding)
	}
	distAmount := (balance - witholding) / uint64(s.NumAccounts)
	utils.Outf(
		"{{yellow}}distributing funds to accounts (1k batch):{{/}} %s %s\n",
		utils.FormatBalance(distAmount, s.H.Controller().Decimals()),
		s.H.Controller().Symbol(),
	)
	accounts := make([]*cli.PrivateKey, s.NumAccounts)
	factories := make([]chain.AuthFactory, s.NumAccounts)
	maxPendingMessages = max(pubsub.MaxPendingMessages, s.TxsPerSecond*2) // ensure we don't block
	dcli, err := rpc.NewWebSocketClient(
		uris[baseName],
		rpc.DefaultHandshakeTimeout,
		maxPendingMessages,
		consts.MTU,
		pubsub.MaxReadMessageSize,
	) // we write the max read
	if err != nil {
		return err
	}

	// Distribute funds (never more than 10x step size outstanding)
	var (
		funds  = map[codec.Address]uint64{}
		fundsL sync.Mutex
	)
	distributor := NewReliableSender(s.NumAccounts, client, dcli, parser, s.MinCapacity, maxPendingMessages, feePerTx)
	for i := 0; i < s.NumAccounts; i++ {
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
		funds[pk.Address] = distAmount
		distributor.Send(factory, getTransfer(pk.Address, true, distAmount, uniqueBytes()))
	}
	if err := distributor.Wait(context.Background()); err != nil {
		return err
	}
	utils.Outf("{{yellow}}distributed funds:{{/}} %d accounts\n", s.NumAccounts)

	// Kickoff txs
	utils.Outf("{{yellow}}starting load test...{{/}}\n")
	utils.Outf("{{yellow}}max tps:{{/}} %d\n", s.TxsPerSecond)
	utils.Outf("{{yellow}}zipf distribution [(v+k)^(-s)] s:{{/}} %.2f {{yellow}}v:{{/}} %.2f\n", s.Zipf.s, s.Zipf.v)
	clients := []*txIssuer{}
	for i := 0; i < len(uriNames); i++ {
		for j := 0; j < s.NumClients; j++ {
			name := uriNames[i]
			cli := rpc.NewJSONRPCClient(uris[name])
			dcli, err := rpc.NewWebSocketClient(
				uris[name],
				rpc.DefaultHandshakeTimeout,
				maxPendingMessages,
				consts.MTU,
				pubsub.MaxReadMessageSize,
			) // we write the max read
			if err != nil {
				return err
			}
			clients = append(clients, &txIssuer{c: cli, d: dcli, name: name, uri: i})
			if len(clients)%25 == 0 && len(clients) > 0 {
				utils.Outf("{{yellow}}initializing connections:{{/}} %d/%d clients\n", len(clients), len(uriNames)*s.NumClients)
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
		pSent            int64
		pSentBytes       int64
		pReceived        int64
		pReceivedBytes   int64

		startRun      = time.Now()
		tpsWindow     = window.Window{}
		expiredWindow = window.Window{} // needed for ttfWindow but not tps
		ttfWindow     = window.Window{} // max int64 is ~5.3M (so can't have more than that per second)
	)
	go func() {
		for {
			select {
			case <-t.C:
				cSent := sent.Load()
				cSentBytes := sentBytes.Load()
				cReceived := received.Load()
				cReceivedBytes := receivedBytes.Load()
				l.Lock()
				cActive := activeAccounts.Len()
				activeAccounts.Clear()

				dropped := pending.SetMin(time.Now().UnixMilli() - pendingExpiryBuffer) // set in the past to allow for delay on connection
				for range dropped {
					// We don't count confirmation time here because we don't know what happened
					droppedTxs++
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

				txsToProcess := int64(0)
				for _, client := range clients {
					txsToProcess += client.d.TxsToProcess()
				}

				if totalTxs > 0 && tpsSum > 0 {
					// tps is only contains transactions that actually made it onchain
					// ttf includes all transactions that made it onchain or expired, but not transactions that returned an error on submission
					utils.Outf(
						"{{yellow}}tps:{{/}} %.2f (pending=%d) {{yellow}}ttf:{{/}} %.2fs {{yellow}}total txs:{{/}} %d (invalid=%.2f%% dropped=%.2f%% failed=%.2f%% expired[all time]=%.2f%% expired[10s]=%.2f%%) {{yellow}}txs sent/s:{{/}} %d (bandwidth=%.2fKB accounts=%d) {{yellow}}txs recv/s:{{/}} %d (bandwidth=%.2fKB backlog=%d)\n", //nolint:lll
						float64(tpsSum)/float64(tpsDivisor),
						pending.Len(),
						float64(window.Sum(ttfWindow))/float64(tpsSum+window.Sum(expiredWindow))/float64(consts.MillisecondsPerSecond),
						totalTxs,
						float64(invalidTxs)/float64(totalTxs)*100,
						float64(droppedTxs)/float64(totalTxs)*100,
						float64(failedTxs)/float64(totalTxs)*100,
						float64(expiredTxs)/float64(totalTxs)*100,
						float64(window.Sum(expiredWindow))/float64(tpsSum+window.Sum(expiredWindow))*100,
						cSent-pSent,
						float64(cSentBytes-pSentBytes)/units.KiB,
						cActive,
						cReceived-pReceived,
						float64(cReceivedBytes-pReceivedBytes)/units.KiB,
						txsToProcess,
					)
				} else if totalTxs > 0 {
					// This shouldn't happen but we should log when it does.
					utils.Outf(
						"{{yellow}}total txs:{{/}} %d (pending=%d invalid=%.2f%% dropped=%.2f%% failed=%.2f%% expired=%.2f%%) {{yellow}}txs sent/s:{{/}} %d (bandwidth=%.2fKB accounts=%d) {{yellow}}txs recv/s:{{/}} %d (bandwidth=%.2fKB backlog=%d)\n", //nolint:lll
						totalTxs,
						pending.Len(),
						float64(invalidTxs)/float64(totalTxs)*100,
						float64(droppedTxs)/float64(totalTxs)*100,
						float64(failedTxs)/float64(totalTxs)*100,
						float64(expiredTxs)/float64(totalTxs)*100,
						cSent-pSent,
						float64(cSentBytes-pSentBytes)/units.KiB,
						cActive,
						cReceived-pReceived,
						float64(cReceivedBytes-pReceivedBytes)/units.KiB,
						txsToProcess,
					)
				}
				l.Unlock()
				pSent = cSent
				pSentBytes = cSentBytes
				pReceived = cReceived
				pReceivedBytes = cReceivedBytes
			case <-cctx.Done():
				return
			}
		}
	}()

	// broadcast txs
	var (
		z = rand.NewZipf(rand.New(rand.NewSource(0)), s.Zipf.s, s.Zipf.v, uint64(s.NumAccounts)-1)

		it                      = time.NewTimer(0)
		currentTarget           = min(s.TxsPerSecond, s.MinCapacity)
		consecutiveUnderBacklog int
		consecutiveAboveBacklog int

		stop bool
	)
	utils.Outf("{{cyan}}initial target tps:{{/}} %d\n", currentTarget)
	for !stop {
		select {
		case <-it.C:
			start := time.Now()

			// Get unprocessed txs (we know about them but haven't been processed, so they aren't outstanding)
			txsToProcess := int64(0)
			for _, client := range clients {
				txsToProcess += client.d.TxsToProcess()
			}

			// Check if we should issue txs
			skipped := false
			if int64(pending.Len()+currentTarget)-txsToProcess > int64(currentTarget*pendingTargetMultiplier) {
				consecutiveAboveBacklog++
				if consecutiveAboveBacklog >= failedRunsToDecreaseTarget {
					if currentTarget > s.StepSize {
						currentTarget -= s.StepSize
						utils.Outf("{{cyan}}skipping issuance because large backlog detected, decreasing target tps:{{/}} %d\n", currentTarget)
					} else {
						utils.Outf("{{cyan}}skipping issuance because large backlog detected, cannot decrease target{{/}}\n")
					}
					consecutiveAboveBacklog = 0
				}
				skipped = true
			} else {
				// Issue txs
				g := &errgroup.Group{}
				g.SetLimit(maxConcurrency)
				thisActive := set.NewSet[codec.Address](currentTarget * 2)
				for i := 0; i < currentTarget; i++ {
					// math.Rand is not safe for concurrent use
					senderIndex, recipientIndex := z.Uint64(), z.Uint64()
					sender := accounts[senderIndex]
					issuerIndex, issuer := getRandomIssuer(clients)
					if recipientIndex == senderIndex {
						if recipientIndex == uint64(s.NumAccounts-1) {
							recipientIndex--
						} else {
							recipientIndex++
						}
					}
					recipient := accounts[recipientIndex].Address
					thisActive.Add(sender.Address, recipient)
					g.Go(func() error {
						factory := factories[senderIndex]
						fundsL.Lock()
						balance := funds[sender.Address]
						if feePerTx > balance {
							fundsL.Unlock()
							utils.Outf("{{orange}}tx has insufficient funds:{{/}} %s\n", codec.MustAddressBech32(s.Hrp, sender.Address))
							return fmt.Errorf("%s has insufficient funds", codec.MustAddressBech32(s.Hrp, sender.Address))
						}
						funds[sender.Address] = balance - feePerTx
						fundsL.Unlock()

						// Send transaction
						action := getTransfer(recipient, false, 1, uniqueBytes())
						if err := issueTransfer(cctx, issuer, issuerIndex, feePerTx, factory, action); err != nil {
							utils.Outf("{{orange}}failed to generate tx (issuer: %d):{{/}} %v\n", issuerIndex, err)
							return err
						}
						return nil
					})
				}
				if err := g.Wait(); err != nil {
					utils.Outf("{{yellow}}exiting broadcast loop because of error:{{/}} %v\n", err)
					cancel()
					stop = true
				}
				l.Lock()
				activeAccounts.Union(thisActive)
				l.Unlock()
			}

			// Determine how long to sleep
			dur := time.Since(start)
			sleep := max(float64(consts.MillisecondsPerSecond-dur.Milliseconds()), 0)
			it.Reset(time.Duration(sleep) * time.Millisecond)

			// Determine next target
			if skipped {
				consecutiveUnderBacklog = 0
				continue
			}
			consecutiveAboveBacklog = 0
			consecutiveUnderBacklog++
			if consecutiveUnderBacklog == successfulRunsToIncreaseTarget && currentTarget < s.TxsPerSecond {
				currentTarget = min(currentTarget+s.StepSize, s.TxsPerSecond)
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
	unitPrices, err = client.UnitPrices(ctx, false)
	if err != nil {
		return err
	}
	feePerTx, err = chain.MulSum(unitPrices, maxUnits)
	if err != nil {
		return err
	}
	utils.Outf("{{yellow}}returning funds to base:{{/}} %s\n", s.H.Controller().Address(key.Address))
	var (
		accountsWithBalance = map[int]uint64{}
		returnedBalance     = uint64(0)
	)
	for i := 0; i < s.NumAccounts; i++ {
		balance := funds[accounts[i].Address]
		if feePerTx > balance {
			continue
		}
		accountsWithBalance[i] = balance
		returnedBalance += balance
	}
	returner := NewReliableSender(len(accountsWithBalance), client, dcli, parser, s.MinCapacity, maxPendingMessages, feePerTx)
	for i, balance := range accountsWithBalance {
		// Send funds
		returnAmt := balance - feePerTx
		returner.Send(factories[i], getTransfer(key.Address, false, returnAmt, uniqueBytes()))
	}
	if err := returner.Wait(context.Background()); err != nil {
		return err
	}
	utils.Outf(
		"{{yellow}}returned funds from %d/%d accounts:{{/}} %s %s\n",
		len(accountsWithBalance),
		s.NumAccounts,
		utils.FormatBalance(returnedBalance, s.H.Controller().Decimals()),
		s.H.Controller().Symbol(),
	)
	return nil
}

// InitializeKey returns the root key and its balance
// If the private key is not set, the user is prompted to select a key
// If the private key is set, the balance is looked up
// The spam script only uses the database here, could do some more decoupling
func (s *Spammer) InitializeRootKey(
	lookupBalance func(int, string) (uint64, error),
) (*cli.PrivateKey, uint64, error) {
	var (
		key     *cli.PrivateKey
		balance uint64
	)
	if s.PrivateKey == nil {
		keys, err := s.H.GetKeys()
		if err != nil {
			return nil, 0, err
		}
		if len(keys) == 0 {
			return nil, 0, ErrNoKeys
		}
		utils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
		balances := make([]uint64, len(keys))
		for i := 0; i < len(keys); i++ {
			address := s.H.Controller().Address(keys[i].Address)
			balance, err := lookupBalance(i, address)
			if err != nil {
				return nil, 0, err
			}
			balances[i] = balance
		}
		keyIndex, err := s.H.PromptChoice("select root key", len(keys))
		if err != nil {
			return nil, 0, err
		}
		key = keys[keyIndex]
		balance = balances[keyIndex]
	} else {
		balanceLookup, err := lookupBalance(-1, s.H.Controller().Address(s.PrivateKey.Address))
		if err != nil {
			return nil, 0, err
		}
		key = s.PrivateKey
		balance = balanceLookup
	}
	
	// No longer using db, so we close
	if err := s.H.CloseDatabase(); err != nil {
		return nil, 0, err
	}

	return key, balance, nil
}

func (s *Spammer) ValidateInputs() error {
	// Distribute funds
	if s.NumAccounts <= 0 {
		_, err := s.H.PromptInt("number of accounts", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if s.TxsPerSecond <= 0 {
		_, err := s.H.PromptInt("txs to try and issue per second", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if s.MinCapacity <= 0 {
		_, err := s.H.PromptInt("minimum txs to issue per second", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if s.StepSize <= 0 {
		_, err := s.H.PromptInt("amount to periodically increase tps by", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	if s.NumClients <= 0 {
		_, err := s.H.PromptInt("number of clients per node", consts.MaxInt)
		if err != nil {
			return err
		}
	}
	return nil
}
