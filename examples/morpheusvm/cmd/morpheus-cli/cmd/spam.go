// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/throughput"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	hthroughput "github.com/ava-labs/hypersdk/throughput"
	"github.com/ava-labs/hypersdk/utils"
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
			_, urisFromFile, err := cli.ReadCLIFile(clusterInfo)
			if err != nil {
				utils.Outf("{{red}} failed to read cluster info: %s \n", err)
				return err
			}
			uris := onlyAPIs(urisFromFile)
			utils.Outf("{{green}}Spamming with %s\n", uris)
			spamConfig, err := hthroughput.NewDefaultCliConfig(uris)
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
		} else {
			panic("cluster info is required")
		}
		// return handler.Root().Spam(ctx, &throughput.SpamHelper{KeyType: args[0]}, spamDefaults)
	},
}

func onlyAPIs(m map[string]string) []string {
	apis := make([]string, 0, len(m))
	for k := range m {
		if !strings.Contains(strings.ToLower(k), "api") {
			continue
		}

		apis = append(apis, m[k])
	}
	return apis
}
