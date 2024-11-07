// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	hthroughput "github.com/ava-labs/hypersdk/throughput"
)

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run [ed25519/secp256r1/bls]",
	PreRunE: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return vm.AuthProvider.CheckType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		privateKey := "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7"
		chains, err := handler.Root().GetChains()
		if err != nil {
			return err
		}

		keys := make([]ids.ID, 0, len(chains))
		for chainID := range chains {
			keys = append(keys, chainID)
		}
		chainIndex := 0
		chainID := keys[chainIndex]
		uris := chains[chainID]
	

		sc := hthroughput.NewDefaultConfig(uris, privateKey)
		spammer := hthroughput.NewSpammer(sc, sh)
	
		return spammer.Spam(ctx, sh, false, h.c.Symbol())
	},
}
