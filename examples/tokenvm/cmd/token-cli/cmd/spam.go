package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/spf13/cobra"
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
		distAmount := balance / uint64(numAccounts+1)
		hutils.Outf("{{yellow}}distributing funds to each account:{{/}} %s %s\n", valueString(ids.Empty, distAmount), assetString(ids.Empty))
		accounts := make([]crypto.PrivateKey, numAccounts)
		txs := make([]ids.ID, numAccounts)
		for i := 0; i < numAccounts; i++ {
			// Create account
			pk, err := crypto.GeneratePrivateKey()
			if err != nil {
				return err
			}
			accounts[i] = pk

			// Send funds
			submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
				To:    pk.PublicKey(),
				Asset: ids.Empty,
				Value: distAmount,
			}, factory)
			if err != nil {
				return err
			}
			if err := submit(ctx); err != nil {
				return err
			}
			txs[i] = tx.ID()
		}
		for i := 0; i < numAccounts; i++ {
			success, err := cli.WaitForTransaction(ctx, txs[i])
			if err != nil {
				return err
			}
			if !success {
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
			ip := net.ParseIP(uris[i])
			tcpURI := fmt.Sprintf("%s:%d", ip.String(), port)
			cli, err := vm.NewDecisionRPCClient(tcpURI)
			if err != nil {
				return err
			}
			clients[i] = &txIssuer{c: c, d: cli}
		}
		t := time.NewTicker(1001 * time.Millisecond) // ensure no duplicates created
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
		for i := 0; i < len(clients); i++ {
			issuer := clients[i]
			wg.Add(1)
			go func() {
				for {
					_, _, result, err := issuer.d.Listen()
					if err != nil {
						return
					}
					issuer.l.Lock()
					issuer.outstandingTxs--
					issuer.l.Unlock()
					l.Lock()
					if result != nil && result.Success {
						confirmedTxs++
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
		for !exiting {
			select {
			case <-t.C:
				for i := 0; i < numAccounts; i++ {
					issuer := getRandomIssuer(clients)
					recipient, err := getRandomRecipient(i, accounts)
					if err != nil {
						return err
					}
					// TODO: make generate transaction more efficient
					_, tx, fees, err := issuer.c.GenerateTransaction(ctx, nil, &actions.Transfer{
						To:    recipient,
						Asset: ids.Empty,
						Value: 1,
					}, auth.NewED25519Factory(accounts[i]))
					if err != nil {
						return err
					}
					transferFee = fees
					if err := issuer.d.IssueTx(tx); err != nil {
						return err
					}
					issuer.l.Lock()
					issuer.outstandingTxs++
					issuer.l.Unlock()
				}
				l.Lock()
				hutils.Outf("{{yellow}}txs broadcast:{{/}} %d {{yellow}}success rate: %.2f{{/}}\n", totalTxs, float64(confirmedTxs)/float64(totalTxs))
				l.Unlock()
			case <-signals:
				hutils.Outf("{{yellow}}exiting spam loop{{/}}\n")
				exiting = true
				cancel()
				break
			}
		}

		// Wait for all issuers to finish
		hutils.Outf("{{yellow}}waiting for issuers to return{{/}}\n")
		wg.Wait()

		// Return funds
		hutils.Outf("{{yellow}}returning funds to %s{{/}}\n", utils.Address(key.PublicKey()))
		var returnedBalance uint64
		txs = make([]ids.ID, numAccounts)
		for i := 0; i < numAccounts; i++ {
			address := utils.Address(accounts[i].PublicKey())
			balance, err := cli.Balance(ctx, address, ids.Empty)
			if err != nil {
				return err
			}
			if transferFee > balance {
				continue
			}
			// Send funds
			returnAmt := balance - transferFee
			submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
				To:    key.PublicKey(),
				Asset: ids.Empty,
				Value: returnAmt,
			}, auth.NewED25519Factory(accounts[i]))
			if err != nil {
				return err
			}
			if err := submit(ctx); err != nil {
				return err
			}
			txs[i] = tx.ID()
			returnedBalance += returnAmt
		}
		for i := 0; i < numAccounts; i++ {
			if txs[i] == ids.Empty {
				// No balance to return
				continue
			}
			success, err := cli.WaitForTransaction(ctx, txs[i])
			if err != nil {
				return err
			}
			if !success {
				// Should never happen
				return ErrTxFailed
			}
		}
		hutils.Outf("{{yellow}}returned balance to %s:{{/}} %s %s\n", utils.Address(key.PublicKey()), valueString(ids.Empty, returnedBalance), assetString(ids.Empty))
		return nil
	},
}
