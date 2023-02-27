// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"math"
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

var fillOrderCmd = &cobra.Command{
	Use:   "fill-order",
	Short: "Fills a new order",
	RunE:  fillOrderFunc,
}

func fillOrderFunc(*cobra.Command, []string) error {
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
	rawInAsset, err := promptText.Run()
	if err != nil {
		return err
	}
	var inAssetID ids.ID
	if rawInAsset != consts.Symbol {
		inAssetID, err = ids.FromString(rawInAsset)
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
	addr := utils.Address(priv.PublicKey())
	balance, err := cli.Balance(ctx, addr, inAssetID)
	if err != nil {
		return err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", inAssetID)
		hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}
	balanceStr := hutils.FormatBalance(balance)
	if inAssetID != ids.Empty {
		// Custom assets are denoted in raw units
		balanceStr = strconv.FormatUint(balance, 10)
	}
	hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", balanceStr, rawInAsset)

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
	rawOutAsset, err := promptText.Run()
	if err != nil {
		return err
	}
	var outAssetID ids.ID
	if rawOutAsset != consts.Symbol {
		outAssetID, err = ids.FromString(rawOutAsset)
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

	// View orders
	orders, err := cli.Orders(ctx, actions.PairID(inAssetID, outAssetID))
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		hutils.Outf("{{red}}no available orders{{/}}\n")
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}
	hutils.Outf("{{cyan}}available orders:{{/}} %d\n", len(orders))
	max := 20
	if len(orders) < max {
		max = len(orders)
	}
	for i := 0; i < max; i++ {
		order := orders[i]
		var inTickStr string
		inTick := float64(order.InTick)
		if inAssetID == ids.Empty {
			inTickStr = hutils.FormatBalance(order.InTick)
			inTick /= math.Pow10(hutils.NativeDecimals)
		} else {
			inTickStr = strconv.FormatUint(order.InTick, 10)
		}
		var outTickStr string
		var remainingStr string
		outTick := float64(order.OutTick)
		if outAssetID == ids.Empty {
			outTickStr = hutils.FormatBalance(order.OutTick)
			remainingStr = hutils.FormatBalance(order.Remaining)
			outTick /= math.Pow10(hutils.NativeDecimals)
		} else {
			outTickStr = strconv.FormatUint(order.OutTick, 10)
			remainingStr = strconv.FormatUint(order.Remaining, 10)
		}
		hutils.Outf(
			"%d) {{cyan}}Rate(in/out):{{/}} %.4f {{cyan}}InTick:{{/}} %s %s {{cyan}}OutTick:{{/}} %s %s {{cyan}}Remaining:{{/}} %s %s\n", //nolint:lll
			i,
			inTick/outTick,
			inTickStr,
			rawInAsset,
			outTickStr,
			rawOutAsset,
			remainingStr,
			rawOutAsset,
		)
	}

	// Select order
	promptText = promptui.Prompt{
		Label: "select order",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			index, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			if index >= max || index < 0 {
				return errors.New("index out of range")
			}
			return nil
		},
	}
	rawOrder, err := promptText.Run()
	if err != nil {
		return err
	}
	orderIndex, err := strconv.Atoi(rawOrder)
	if err != nil {
		return err
	}
	order := orders[orderIndex]

	// Select supply
	promptText = promptui.Prompt{
		Label: "value (must be multiple of InTick)",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			var amount uint64
			var err error
			if inAssetID == ids.Empty {
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
			if amount%order.InTick != 0 {
				return errors.New("must be multiple of inTick")
			}
			multiples := amount / order.InTick
			requiredRemainder := order.OutTick * multiples
			if requiredRemainder > order.Remaining {
				return errors.New("not enough left")
			}
			return nil
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return err
	}
	var value uint64
	if inAssetID == ids.Empty {
		value, err = hutils.ParseBalance(rawAmount)
	} else {
		value, err = strconv.ParseUint(rawAmount, 10, 64)
	}
	if err != nil {
		return err
	}
	multiples := value / order.InTick
	outAmount := multiples * order.OutTick
	var outStr string
	if outAssetID == ids.Empty {
		outStr = hutils.FormatBalance(outAmount)
	} else {
		outStr = strconv.FormatUint(outAmount, 10)
	}
	hutils.Outf(
		"{{orange}}in:{{/}} %s %s {{orange}}out:{{/}} %s %s\n",
		rawAmount,
		rawInAsset,
		outStr,
		rawOutAsset,
	)

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

	owner, err := utils.ParseAddress(order.Owner)
	if err != nil {
		return err
	}
	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.FillOrder{
		Order: order.ID,
		Owner: owner,
		In:    inAssetID,
		Out:   outAssetID,
		Value: value,
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
	hutils.Outf("{{yellow}}txID:{{/}} %s\n", tx.ID())
	return nil
}
