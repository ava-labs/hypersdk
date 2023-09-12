// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/challenge"
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
	hutils.Outf(
		"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
		choice,
		address,
		hutils.FormatBalance(balance, tconsts.Decimals),
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
		_, priv, _, _, _, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}
		faucet, err := tcli.FaucetAddress(ctx)
		if err != nil {
			return err
		}
		salt, difficulty, err := tcli.Challenge(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{yellow}}searching for faucet solutions (difficulty=%d, faucet=%s):{{/}} %x\n", difficulty, faucet, salt)
		start := time.Now()
		solution, attempts := challenge.Search(salt, difficulty, numCores)
		utils.Outf("{{cyan}}found solution (attempts=%d, t=%s):{{/}} %x\n", attempts, time.Since(start), solution)
		txID, err := tcli.SolveChallenge(ctx, tutils.Address(priv.PublicKey()), salt, solution)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}fauceted funds incoming:{{/}} %s\n", txID)
		return nil
	},
}
