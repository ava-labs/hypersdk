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
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfers value to another address",
	RunE:  transferFunc,
}

func transferFunc(_ *cobra.Command, args []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	factory := auth.NewED25519Factory(priv)

	ctx := context.Background()
	cli := client.New(uri)

	// Select token to send
	f := func(input string) error {
		if len(input) == 0 {
			return errors.New("input is empty")
		}
		if len(input) == 3 && input == "TKN" {
			return nil
		}
		_, err := ids.FromString(input)
		return err
	}
	promptText := promptui.Prompt{
		Label:    "asset (use TKN for native token)",
		Validate: f,
	}
	asset, err := promptText.Run()
	if err != nil {
		return err
	}
	var assetID ids.ID
	if asset == "TKN" {
		assetID = ids.Empty
	}
	addr := utils.Address(priv.PublicKey())
	balance, err := cli.Balance(ctx, addr, assetID)
	if err != nil {
		return err
	}
	if balance == 0 {
		hutils.Outf("{{red}}balance:{{/}} 0 %s\n", asset)
		hutils.Outf("{{red}}please send funds to %s...exiting{{/}}/n", addr)
		return nil
	}
	balanceStr := hutils.FormatBalance(balance)
	if assetID != ids.Empty {
		// Custom assets are denoted in raw units
		balanceStr = strconv.FormatUint(balance, 10)
	}
	hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", balanceStr, asset)

	// Select recipient
	promptText = promptui.Prompt{
		Label: "recipient",
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

	// Select amount
	promptText = promptui.Prompt{
		Label: "amount",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			// TODO: ensure can't send more than balance
			if assetID == ids.Empty {
				_, err := hutils.ParseBalance(input)
				return err
			}
			_, err := strconv.ParseUint(input, 10, 64)
			return err
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return err
	}
	var amount uint64
	if assetID == ids.Empty {
		amount, err = hutils.ParseBalance(rawAmount)
	} else {
		amount, err = strconv.ParseUint(rawAmount, 10, 64)
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

	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.Transfer{
		To:    pk,
		Asset: assetID,
		Value: amount,
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
	hutils.Outf("{{yellow}}TxID:{{/}} %s\n", tx.ID())
	return nil
}
