// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/hypersdk/crypto"
	hutils "github.com/ava-labs/hypersdk/utils"
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

var setKeyCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		keys, err := GetKeys()
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			hutils.Outf("{{red}}no stored keys{{/}}\n")
			return nil
		}
		hutils.Outf("{{cyan}}stored keys:{{/}} %d\n", len(keys))
		for i := 0; i < len(keys); i++ {
			publicKey := keys[i].PublicKey()
			hutils.Outf(
				"%d) {{cyan}}address:{{/}} %s {{cyan}}public key:{{/}} %x\n",
				i,
				utils.Address(publicKey),
				publicKey,
			)
		}

		// Select key
		keyIndex, err := promptChoice("set default key", len(keys))
		if err != nil {
			return err
		}
		key := keys[keyIndex]
		publicKey := key.PublicKey()
		return StoreDefault(defaultKeyKey, publicKey[:])
	},
}

var balanceKeyCmd = &cobra.Command{
	Use: "balance",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		priv, _, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		assetID, err := promptAsset("assetID", true)
		if err != nil {
			return err
		}
		_, err = getAssetInfo(ctx, cli, priv.PublicKey(), assetID, true)
		return err
	},
}
