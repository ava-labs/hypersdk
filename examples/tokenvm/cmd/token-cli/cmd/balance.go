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
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Balance of a given address",
	RunE:  balanceFunc,
}

func balanceFunc(*cobra.Command, []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	hutils.Outf("{{yellow}}loaded address:{{/}} %s\n\n", utils.Address(priv.PublicKey()))

	ctx := context.Background()
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
}
