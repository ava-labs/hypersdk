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
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/listeners"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/spf13/cobra"
)

const feePerTx = 1000

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
			exiting     bool
			wg          sync.WaitGroup

			l            sync.Mutex
			confirmedTxs uint64
			totalTxs     uint64
		)

		// confirm txs (track failure rate)
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()
		inflightTxs := set.NewSet[ids.ID](numAccounts)
		var infl sync.Mutex
		for i := 0; i < len(clients); i++ {
			issuer := clients[i]
			wg.Add(1)
			go func() {
				for {
					txID, dErr, result, err := issuer.d.Listen()
					if err != nil {
						return
					}
					infl.Lock()
					inflightTxs.Remove(txID)
					infl.Unlock()
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

		// broadcast txs
		t := time.NewTimer(0) // ensure no duplicates created
		defer t.Stop()
		var runs int
		for !exiting {
			select {
			case <-t.C:
				// Ensure we aren't too backlogged
				infl.Lock()
				inflight := inflightTxs.Len()
				infl.Unlock()
				if inflight > maxTxBacklog {
					t.Reset(1 * time.Second)
					continue
				}

				// Generate new transactions
				start := time.Now()
				for i := 0; i < numAccounts; i++ {
					var (
						issuer = getRandomIssuer(clients)
						tx     *chain.Transaction
						fees   uint64
					)
					for {
						recipient, err := getRandomRecipient(i, accounts)
						if err != nil {
							return err
						}
						_, tx, fees, err = issuer.c.GenerateTransaction(ctx, nil, &actions.Transfer{
							To:    recipient,
							Asset: ids.Empty,
							Value: 1,
						}, auth.NewED25519Factory(accounts[i]))
						if err != nil {
							hutils.Outf("{{orange}}failed to generate:{{/}} %v\n", err)
							continue
						}
						infl.Lock()
						exit := !inflightTxs.Contains(tx.ID())
						infl.Unlock()
						if exit {
							break
						}
					}
					transferFee = fees
					infl.Lock()
					inflightTxs.Add(tx.ID())
					infl.Unlock()
					if err := issuer.d.IssueTx(tx); err != nil {
						infl.Lock()
						inflightTxs.Remove(tx.ID())
						infl.Unlock()

						hutils.Outf("{{orange}}failed to issue:{{/}} %v\n", err)
						continue
					}
					funds[accounts[i].PublicKey()] -= (fees + 1)
					issuer.l.Lock()
					issuer.outstandingTxs++
					issuer.l.Unlock()

					// Only send 1 transaction per second until we are sure Snowman++ is
					// activated.
					if runs < 10 {
						runs++
						break
					}
				}
				l.Lock()
				infl.Lock()
				if totalTxs > 0 {
					hutils.Outf(
						"{{yellow}}txs seen:{{/}} %d {{yellow}}success rate:{{/}} %.2f%% {{yellow}}inflight:{{/}} %d\n",
						totalTxs,
						float64(confirmedTxs)/float64(totalTxs)*100,
						inflightTxs.Len(),
					)
				}
				infl.Unlock()
				l.Unlock()

				// Limit the script to looping no more than once a second
				dur := time.Since(start)
				sleep := math.Max(1000-dur.Milliseconds(), 0)
				t.Reset(time.Duration(sleep) * time.Millisecond)
			case <-signals:
				hutils.Outf("{{yellow}}exiting broadcast loop{{/}}\n")
				exiting = true
				cancel()
			}
		}

		// Wait for all issuers to finish
		hutils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
		dctx, cancel := context.WithCancel(ctx)
		go func() {
			// Send a dummy transaction if shutdown is taking too long (listeners are
			// expired on accept if dropped)
			t := time.NewTicker(30 * time.Second)
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
			sent            int
		)
		for i := 0; i < numAccounts; i++ {
			balance := funds[accounts[i].PublicKey()]
			if transferFee > balance {
				continue
			}
			sent++
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
		for i := 0; i < sent; i++ {
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
