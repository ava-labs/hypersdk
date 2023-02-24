// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var createOrderCmd = &cobra.Command{
	Use:   "create-order",
	Short: "Creates a new order",
	RunE:  createOrderFunc,
}

func createOrderFunc(*cobra.Command, []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	factory := auth.NewED25519Factory(priv)
	hutils.Outf("{{yellow}}loaded address:{{/}} %s\n\n", utils.Address(priv.PublicKey()))

	ctx := context.Background()
	cli := client.New(uri)

	// Select inbound token
	promptText := promptui.Prompt{
		Label: "in assetID (use TKN for native token)",
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
	rawAsset, err := promptText.Run()
	if err != nil {
		return err
	}
	var inAssetID ids.ID
	if rawAsset != consts.Symbol {
		inAssetID, err = ids.FromString(rawAsset)
		if err != nil {
			return err
		}
	}
	if inAssetID != ids.Empty {
		exists, metadata, supply, _, err := cli.Asset(ctx, inAssetID)
		if err != nil {
			return err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", inAssetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf(
			"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
			string(metadata),
			supply,
		)
	}

	// Select in tick
	promptText = promptui.Prompt{
		Label: "in tick",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			var err error
			if inAssetID == ids.Empty {
				_, err = hutils.ParseBalance(input)
			} else {
				_, err = strconv.ParseUint(input, 10, 64)
			}
			return err
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return err
	}
	var inTick uint64
	if inAssetID == ids.Empty {
		inTick, err = hutils.ParseBalance(rawAmount)
	} else {
		inTick, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	if err != nil {
		return err
	}

	// Select outbound token
	promptText = promptui.Prompt{
		Label: "out assetID (use TKN for native token)",
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
	rawAsset, err = promptText.Run()
	if err != nil {
		return err
	}
	var outAssetID ids.ID
	if rawAsset != consts.Symbol {
		outAssetID, err = ids.FromString(rawAsset)
		if err != nil {
			return err
		}
	}
	if outAssetID != ids.Empty {
		exists, metadata, supply, _, err := cli.Asset(ctx, outAssetID)
		if err != nil {
			return err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", outAssetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf(
			"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
			string(metadata),
			supply,
		)
	}
	addr := utils.Address(priv.PublicKey())
	balance, err := cli.Balance(ctx, addr, outAssetID)
	if err != nil {
		return err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", outAssetID)
		hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}
	balanceStr := hutils.FormatBalance(balance)
	if outAssetID != ids.Empty {
		// Custom assets are denoted in raw units
		balanceStr = strconv.FormatUint(balance, 10)
	}
	hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", balanceStr, rawAsset)

	// Select out tick
	promptText = promptui.Prompt{
		Label: "out tick",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			var err error
			if outAssetID == ids.Empty {
				_, err = hutils.ParseBalance(input)
			} else {
				_, err = strconv.ParseUint(input, 10, 64)
			}
			return err
		},
	}
	rawAmount, err = promptText.Run()
	if err != nil {
		return err
	}
	var outTick uint64
	if outAssetID == ids.Empty {
		outTick, err = hutils.ParseBalance(rawAmount)
	} else {
		outTick, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	if err != nil {
		return err
	}

	// Select supply
	promptText = promptui.Prompt{
		Label: "supply (must be multiple of OutTick)",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			var amount uint64
			var err error
			if outAssetID == ids.Empty {
				amount, err = hutils.ParseBalance(input)
			} else {
				amount, err = strconv.ParseUint(input, 10, 64)
			}
			if err != nil {
				return err
			}
			if amount > balance {
				return errors.New("insufficient balance")
			}
			if amount%outTick != 0 {
				return errors.New("must be multiple of outTick")
			}
			return nil
		},
	}
	rawAmount, err = promptText.Run()
	if err != nil {
		return err
	}
	var supply uint64
	if outAssetID == ids.Empty {
		supply, err = hutils.ParseBalance(rawAmount)
	} else {
		supply, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	if err != nil {
		return err
	}

	// Confirm action
	promptText = promptui.Prompt{
		Label: "continue (y/n)",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			lower := strings.ToLower(input)
			if lower == "y" || lower == "n" {
				return nil
			}
			return errors.New("invalid choice")
		},
	}
	rawContinue, err := promptText.Run()
	if err != nil {
		return err
	}
	cont := strings.ToLower(rawContinue)
	if cont == "n" {
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}

	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.CreateOrder{
		In:      inAssetID,
		InTick:  inTick,
		Out:     outAssetID,
		OutTick: outTick,
		Supply:  supply,
	}, factory)
	if err != nil {
		return err
	}
	if err := submit(ctx); err != nil {
		return err
	}
	success, err := cli.WaitForTransaction(ctx, tx.ID())
	if err != nil {
		return err
	}
	if success {
		hutils.Outf("{{green}}transaction succeeded{{/}}\n")
	} else {
		hutils.Outf("{{red}}transaction failed{{/}}\n")
	}
	hutils.Outf("{{yellow}}orderID:{{/}} %s\n", tx.ID())
	return nil
}
