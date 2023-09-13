// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var keyCmd = &cobra.Command{
	Use: "key",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genKeyCmd = &cobra.Command{
	Use: "generate",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().GenerateKey()
	},
}

var importKeyCmd = &cobra.Command{
	Use: "import [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportKey(args[0])
	},
}

func lookupSetKeyBalance(choice int, address string, uri string, networkID uint32, chainID ids.ID) error {
	// TODO: just load once
	cli := trpc.NewJSONRPCClient(uri, networkID, chainID)
	balance, err := cli.Balance(context.TODO(), address, ids.Empty)
	if err != nil {
		return err
	}
	utils.Outf(
		"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
		choice,
		address,
		utils.FormatBalance(balance, tconsts.Decimals),
		tconsts.Symbol,
	)
	return nil
}

var setKeyCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().SetKey(lookupSetKeyBalance)
	},
}

func lookupKeyBalance(pk ed25519.PublicKey, uri string, networkID uint32, chainID ids.ID, assetID ids.ID) error {
	_, _, _, _, err := handler.GetAssetInfo(
		context.TODO(), trpc.NewJSONRPCClient(uri, networkID, chainID),
		pk, assetID, true)
	return err
}

var balanceKeyCmd = &cobra.Command{
	Use: "balance",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().Balance(checkAllChains, true, lookupKeyBalance)
	},
}

var faucetKeyCmd = &cobra.Command{
	Use: "faucet",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()

		// Get private key
		_, priv, _, _, _, _, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Get faucet
		faucetURI, err := handler.Root().PromptString("faucet URI", 0, consts.MaxInt)
		if err != nil {
			return err
		}
		fcli := frpc.NewJSONRPCClient(faucetURI)
		faucet, err := fcli.FaucetAddress(ctx)
		if err != nil {
			return err
		}

		// Search for funds
		salt, difficulty, err := fcli.Challenge(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{yellow}}searching for faucet solutions (difficulty=%d, faucet=%s):{{/}} %x\n", difficulty, faucet, salt)
		start := time.Now()
		solution, attempts := challenge.Search(salt, difficulty, numCores)
		utils.Outf("{{cyan}}found solution (attempts=%d, t=%s):{{/}} %x\n", attempts, time.Since(start), solution)
		txID, amount, err := fcli.SolveChallenge(ctx, tutils.Address(priv.PublicKey()), salt, solution)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}faucet funds incoming (%s %s):{{/}} %s\n", utils.FormatBalance(amount, tconsts.Decimals), tconsts.Symbol, txID)
		return nil
	},
}
