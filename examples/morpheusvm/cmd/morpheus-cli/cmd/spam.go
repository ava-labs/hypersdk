// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/throughput"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/utils"

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

		if len(clusterInfo) > 0 {
			_, urisFromFile, err := cli.ReadClusterInfoFile(clusterInfo)
			if err != nil {
				utils.Outf("{{red}} failed to read cluster info: %s \n", err)
				return err
			}
			uris := cli.OnlyAPIs(urisFromFile)

			if len(uris) == 0 || len(spamKey) == 0 {
				return errors.New("uris and keyHex must be non-empty")
			}
			bytes, err := codec.LoadHex(spamKey, ed25519.PrivateKeyLen)
			if err != nil {
				return err
			}
			authFactory := auth.NewED25519Factory(ed25519.PrivateKey(bytes))

			spamConfig, err := hthroughput.NewLongRunningConfig(uris, authFactory)
			if err != nil {
				return err
			}

			spamHelper := &throughput.SpamHelper{KeyType: args[0]}
			if err := spamHelper.CreateClient(uris[0]); err != nil {
				return err
			}
			spammer, err := hthroughput.NewSpammer(spamConfig, spamHelper)
			if err != nil {
				return err
			}
			return spammer.Spam(ctx, spamHelper, true, "AVAX")
		}

		return handler.Root().Spam(ctx, &throughput.SpamHelper{KeyType: args[0]}, spamKey, spamDefaults)
	},
}
