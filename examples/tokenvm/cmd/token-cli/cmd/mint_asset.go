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

var mintAssetCmd = &cobra.Command{
	Use:   "mint-asset",
	Short: "Mints a new asset",
	RunE:  mintAssetFunc,
}

func mintAssetFunc(*cobra.Command, []string) error {
	priv, err := crypto.LoadKey(privateKeyFile)
	if err != nil {
		return err
	}
	factory := auth.NewED25519Factory(priv)
	hutils.Outf("{{yellow}}loaded address:{{/}} %s\n\n", utils.Address(priv.PublicKey()))

	ctx := context.Background()
	cli := client.New(uri)

	// Select token to mint
	promptText := promptui.Prompt{
		Label: "assetID",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			assetID, err := ids.FromString(input)
			if err != nil {
				return err
			}
			if assetID == ids.Empty {
				return errors.New("cannot mint native")
			}
			return nil
		},
	}
	rawAsset, err := promptText.Run()
	if err != nil {
		return err
	}
	assetID, err := ids.FromString(rawAsset)
	if err != nil {
		return err
	}
	exists, metadata, supply, owner, err := cli.Asset(ctx, assetID)
	if err != nil {
		return err
	}
	if !exists {
		hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}
	if owner != utils.Address(priv.PublicKey()) {
		hutils.Outf("{{red}}%s is the owner of %s, you are not{{/}}\n", owner, assetID)
		hutils.Outf("{{red}}exiting...{{/}}\n")
		return nil
	}
	hutils.Outf("{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n", string(metadata), supply)

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
			_, err := strconv.ParseUint(input, 10, 64)
			return err
		},
	}
	rawAmount, err := promptText.Run()
	if err != nil {
		return err
	}
	amount, err := strconv.ParseUint(rawAmount, 10, 64)
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

	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.MintAsset{
		Asset: assetID,
		To:    pk,
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
	hutils.Outf("{{yellow}}txID:{{/}} %s\n", tx.ID())
	return nil
}
