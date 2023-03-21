package cmd

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

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

func getRandomClient(clis []*client.Client) *client.Client {
	index := rand.Int() % len(clis)
	return clis[index]
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
		clients := make([]*client.Client, len(uris))
		for i := 0; i < len(uris); i++ {
			clients[i] = client.New(uris[i])
		}
		t := time.NewTicker(1001 * time.Millisecond) // ensure no duplicates created
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		var (
			transferFee uint64
			exiting     bool
		)
		for !exiting {
			select {
			case <-t.C:
				for i := 0; i < numAccounts; i++ {
					c := getRandomClient(clients)
					recipient, err := getRandomRecipient(i, accounts)
					if err != nil {
						return err
					}
					submit, _, fees, err := c.GenerateTransaction(ctx, nil, &actions.Transfer{
						To:    recipient,
						Asset: ids.Empty,
						Value: 1,
					}, auth.NewED25519Factory(accounts[i]))
					if err != nil {
						return err
					}
					transferFee = fees
					if err := submit(ctx); err != nil {
						return err
					}
				}
			case <-signals:
				hutils.Outf("{{yellow}}exiting creation loop:{{/}} %v\n", ctx.Err())
				exiting = true
				break
			}
		}

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
