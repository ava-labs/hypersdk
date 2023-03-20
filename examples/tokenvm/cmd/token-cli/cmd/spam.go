package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
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
		for i := 0; i < len(keys); i++ {
			address := utils.Address(keys[i].PublicKey())
			balance, err := cli.Balance(context.TODO(), address, ids.Empty)
			if err != nil {
				return err
			}
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

		// Distribute funds
		accounts, err := promptInt("number of accounts")
		if err != nil {
			return err
		}

		// Kickoff txs

		// Return funds
		return nil
	},
}
