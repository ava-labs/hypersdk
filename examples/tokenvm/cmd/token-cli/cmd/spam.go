// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:gosec
package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/listeners"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const (
	feePerTx     = 1000
	defaultRange = 32
)

type txIssuer struct {
	c *client.Client
	d *vm.DecisionRPCClient

	l              sync.Mutex
	outstandingTxs int
}

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

func getRandomRecipient(self int, keys []crypto.PrivateKey) (crypto.PublicKey, error) {
	if randomRecipient {
		priv, err := crypto.GeneratePrivateKey()
		if err != nil {
			return crypto.EmptyPublicKey, err
		}
		return priv.PublicKey(), nil
	}

	// Select item from array
	index := rand.Int() % len(keys)
	if index == self {
		index++
		if index == len(keys) {
			index = 0
		}
	}
	return keys[index].PublicKey(), nil
}

func getRandomIssuer(issuers []*txIssuer) *txIssuer {
	index := rand.Int() % len(issuers)
	return issuers[index]
}

var runSpamCmd = &cobra.Command{
	Use: "run",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()

		// Select chain
		_, uris, err := promptChain("select chainID", nil)
		if err != nil {
			return err
		}

		// Select root key
		keys, err := GetKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return ErrNoKeys
		}
		cli := client.New(uris[0])
		hutils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
		balances := make([]uint64, len(keys))
		for i := 0; i < len(keys); i++ {
			address := utils.Address(keys[i].PublicKey())
			balance, err := cli.Balance(ctx, address, ids.Empty)
			if err != nil {
				return err
			}
			balances[i] = balance
			hutils.Outf(
				"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s TKN\n",
				i,
				address,
				valueString(ids.Empty, balance),
			)
		}
		keyIndex, err := promptChoice("select root key", len(keys))
		if err != nil {
			return err
		}
		key := keys[keyIndex]
		balance := balances[keyIndex]
		factory := auth.NewED25519Factory(key)

		// Distribute funds
		numAccounts, err := promptInt("number of accounts")
		if err != nil {
			return err
		}
		numTxsPerAccount, err := promptInt("number of transactions per account per second")
		if err != nil {
			return err
		}
		witholding := uint64(feePerTx * numAccounts)
		distAmount := (balance - witholding) / uint64(numAccounts)
		hutils.Outf(
			"{{yellow}}distributing funds to each account:{{/}} %s %s\n",
			valueString(ids.Empty, distAmount),
			assetString(ids.Empty),
		)
		accounts := make([]crypto.PrivateKey, numAccounts)
		port, err := cli.DecisionsPort(ctx)
		if err != nil {
			return err
		}
		u, err := url.Parse(uris[0])
		if err != nil {
			return err
		}
		tcpURI := fmt.Sprintf("%s:%d", u.Hostname(), port)
		dcli, err := vm.NewDecisionRPCClient(tcpURI)
		if err != nil {
			return err
		}
		funds := map[crypto.PublicKey]uint64{}
		var fundsL sync.Mutex
		for i := 0; i < numAccounts; i++ {
			// Create account
			pk, err := crypto.GeneratePrivateKey()
			if err != nil {
				return err
			}
			accounts[i] = pk

			// Send funds
			_, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
				To:    pk.PublicKey(),
				Asset: ids.Empty,
				Value: distAmount,
			}, factory)
			if err != nil {
				return err
			}
			if err := dcli.IssueTx(tx); err != nil {
				return err
			}
			funds[pk.PublicKey()] = distAmount

			// Ensure Snowman++ is activated
			if i < 10 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		for i := 0; i < numAccounts; i++ {
			_, dErr, result, err := dcli.Listen()
			if err != nil {
				return err
			}
			if dErr != nil {
				return dErr
			}
			if !result.Success {
				// Should never happen
				return ErrTxFailed
			}
		}
		hutils.Outf("{{yellow}}distributed funds to %d accounts{{/}}\n", numAccounts)

		// Kickoff txs
		clients := make([]*txIssuer, len(uris))
		for i := 0; i < len(uris); i++ {
			c := client.New(uris[i])
			port, err := c.DecisionsPort(ctx)
			if err != nil {
				return err
			}
			u, err := url.Parse(uris[i])
			if err != nil {
				return err
			}
			tcpURI := fmt.Sprintf("%s:%d", u.Hostname(), port)
			cli, err := vm.NewDecisionRPCClient(tcpURI)
			if err != nil {
				return err
			}
			clients[i] = &txIssuer{c: c, d: cli}
		}
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		var (
			transferFee uint64
			wg          sync.WaitGroup

			l            sync.Mutex
			confirmedTxs uint64
			totalTxs     uint64
		)

		// confirm txs (track failure rate)
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()
		var inflight atomic.Int64
		var sent atomic.Int64
		var exiting sync.Once
		for i := 0; i < len(clients); i++ {
			issuer := clients[i]
			wg.Add(1)
			go func() {
				for {
					_, dErr, result, err := issuer.d.Listen()
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
							hutils.Outf("{{orange}}on-chain tx failure:{{/}} %s %t\n", string(result.Output), result.Success)
						}
					} else {
						// We can't error match here because we receive it over the wire.
						if !strings.Contains(dErr.Error(), listeners.ErrExpired.Error()) {
							hutils.Outf("{{orange}}pre-execute tx failure:{{/}} %v\n", dErr)
						}
					}
					totalTxs++
					l.Unlock()
				}
			}()
			go func() {
				<-cctx.Done()
				for {
					issuer.l.Lock()
					outstanding := issuer.outstandingTxs
					issuer.l.Unlock()
					if outstanding == 0 {
						_ = issuer.d.Close()
						wg.Done()
						return
					}
					time.Sleep(500 * time.Millisecond)
				}
			}()
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
						hutils.Outf(
							"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d {{yellow}}issued/s:{{/}} %d\n",
							totalTxs,
							float64(confirmedTxs)/float64(totalTxs)*100,
							inflight.Load(),
							current-psent,
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
		genesis, err := clients[0].c.Genesis(ctx)
		if err != nil {
			return err
		}
		_, _, chainID, err := clients[0].c.Network(ctx)
		if err != nil {
			return err
		}
		unitPrice, _, err := clients[0].c.SuggestedRawFee(ctx)
		if err != nil {
			return err
		}
		g, gctx := errgroup.WithContext(ctx)
		for ri := 0; ri < numAccounts; ri++ {
			i := ri
			g.Go(func() error {
				t := time.NewTimer(0) // ensure no duplicates created
				defer t.Stop()

				issuer := getRandomIssuer(clients)
				factory := auth.NewED25519Factory(accounts[i])
				for {
					select {
					case <-t.C:
						// Ensure we aren't too backlogged
						if inflight.Load() > int64(maxTxBacklog) {
							t.Reset(1 * time.Second)
							continue
						}

						// Send transaction
						start := time.Now()
						for k := 0; k < numTxsPerAccount; k++ {
							recipient, err := getRandomRecipient(i, accounts)
							if err != nil {
								return err
							}
							_, tx, fees, err := issuer.c.GenerateTransactionManual(ctx, genesis, chainID, nil, &actions.Transfer{
								To:    recipient,
								Asset: ids.Empty,
								Value: 1,
							}, factory, unitPrice)
							if err != nil {
								hutils.Outf("{{orange}}failed to generate:{{/}} %v\n", err)
								continue
							}
							transferFee = fees
							if err := issuer.d.IssueTx(tx); err != nil {
								continue
							}
							fundsL.Lock()
							funds[accounts[i].PublicKey()] -= (fees + 1)
							fundsL.Unlock()
							issuer.l.Lock()
							issuer.outstandingTxs++
							issuer.l.Unlock()
							inflight.Add(1)
							sent.Add(1)
						}

						// Determine how long to sleep
						dur := time.Since(start)
						sleep := math.Max(1000-dur.Milliseconds(), 0)
						t.Reset(time.Duration(sleep) * time.Millisecond)
					case <-gctx.Done():
						return gctx.Err()
					case <-cctx.Done():
						return nil
					case <-signals:
						exiting.Do(func() {
							hutils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
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
		hutils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
		dctx, cancel := context.WithCancel(ctx)
		go func() {
			// Send a dummy transaction if shutdown is taking too long (listeners are
			// expired on accept if dropped)
			t := time.NewTicker(15 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					_ = submitDummy(dctx, cli, key.PublicKey(), factory)
				case <-dctx.Done():
					return
				}
			}
		}()
		wg.Wait()
		cancel()

		// Return funds
		hutils.Outf("{{yellow}}returning funds to %s{{/}}\n", utils.Address(key.PublicKey()))
		var (
			returnedBalance uint64
			returnsSent     int
		)
		for i := 0; i < numAccounts; i++ {
			balance := funds[accounts[i].PublicKey()]
			if transferFee > balance {
				continue
			}
			returnsSent++
			// Send funds
			returnAmt := balance - transferFee
			_, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
				To:    key.PublicKey(),
				Asset: ids.Empty,
				Value: returnAmt,
			}, auth.NewED25519Factory(accounts[i]))
			if err != nil {
				return err
			}
			if err := dcli.IssueTx(tx); err != nil {
				return err
			}
			returnedBalance += returnAmt

			// Ensure Snowman++ is activated
			if i < 10 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		for i := 0; i < returnsSent; i++ {
			_, dErr, result, err := dcli.Listen()
			if err != nil {
				return err
			}
			if dErr != nil {
				return dErr
			}
			if !result.Success {
				// Should never happen
				return ErrTxFailed
			}
		}
		hutils.Outf(
			"{{yellow}}returned funds:{{/}} %s %s\n",
			valueString(ids.Empty, returnedBalance),
			assetString(ids.Empty),
		)
		return nil
	},
}
