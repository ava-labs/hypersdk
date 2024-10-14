// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/throughput"
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
		return auth.CheckKeyType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		return handler.Root().Spam(ctx, &throughput.SpamHelper{KeyType: args[0]}, spamDefaults)
	},
}
