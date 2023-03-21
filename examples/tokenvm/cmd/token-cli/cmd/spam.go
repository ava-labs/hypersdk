package cmd

import (
	"context"

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
			success, err := cli.WaitForTransaction(ctx, tx.ID())
			if err != nil {
				return err
			}
			if !success {
				// Should never happen
				return ErrTxFailed
			}
		}

		// Kickoff txs

		// Return funds
		return nil
	},
}
