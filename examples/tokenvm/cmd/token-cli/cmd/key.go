// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
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
		// TODO: encrypt key
		priv, err := crypto.GeneratePrivateKey()
		if err != nil {
			return err
		}
		if err := StoreKey(priv); err != nil {
			return err
		}
		publicKey := priv.PublicKey()
		if err := StoreDefault(defaultKeyKey, publicKey[:]); err != nil {
			return err
		}
		color.Green(
			"created address %s",
			utils.Address(publicKey),
		)
		return nil
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
		priv, err := crypto.LoadKey(args[0])
		if err != nil {
			return err
		}
		if err := StoreKey(priv); err != nil {
			return err
		}
		publicKey := priv.PublicKey()
		if err := StoreDefault(defaultKeyKey, publicKey[:]); err != nil {
			return err
		}
		color.Green(
			"imported address %s",
			utils.Address(publicKey),
		)
		return nil
	},
}
