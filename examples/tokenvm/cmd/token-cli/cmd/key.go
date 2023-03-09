// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
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
		promptText := promptui.Prompt{
			Label: "set default key",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("input is empty")
				}
				index, err := strconv.Atoi(input)
				if err != nil {
					return err
				}
				if index >= len(keys) {
					return errors.New("index out of range")
				}
				return nil
			},
		}
		rawKey, err := promptText.Run()
		if err != nil {
			return err
		}
		keyIndex, err := strconv.Atoi(rawKey)
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
		priv, err := GetDefaultKey()
		if err != nil {
			return err
		}
		if priv == crypto.EmptyPrivateKey {
			return nil
		}
		hutils.Outf("{{yellow}}loaded address:{{/}} %s\n\n", utils.Address(priv.PublicKey()))

		uri, err := GetDefaultChain()
		if err != nil {
			return err
		}
		if len(uri) == 0 {
			return nil
		}
		cli := client.New(uri)

		// Select address
		promptText := promptui.Prompt{
			Label: "address",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("input is empty")
				}
				_, err := utils.ParseAddress(input)
				return err
			},
		}
		recipient, err := promptText.Run()
		if err != nil {
			return err
		}
		pk, err := utils.ParseAddress(recipient)
		if err != nil {
			return err
		}
		addr := utils.Address(pk)

		// Select token to check
		promptText = promptui.Prompt{
			Label: "assetID (use TKN for native token)",
			Validate: func(input string) error {
				if len(input) == 0 {
					return errors.New("input is empty")
				}
				if len(input) == 3 && input == consts.Symbol {
					return nil
				}
				_, err := ids.FromString(input)
				return err
			},
		}
		asset, err := promptText.Run()
		if err != nil {
			return err
		}
		var assetID ids.ID
		if asset != consts.Symbol {
			assetID, err = ids.FromString(asset)
			if err != nil {
				return err
			}
		}

		balance, err := cli.Balance(ctx, addr, assetID)
		if err != nil {
			return err
		}
		balanceStr := hutils.FormatBalance(balance)
		if assetID != ids.Empty {
			// Custom assets are denoted in raw units
			balanceStr = strconv.FormatUint(balance, 10)
		}
		hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", balanceStr, asset)
		return nil
	},
}
