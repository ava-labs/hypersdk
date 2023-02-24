// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
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

var closeOrderCmd = &cobra.Command{
	Use:   "close-order",
	Short: "Closes an existing order",
	RunE:  closeOrderFunc,
}

func closeOrderFunc(*cobra.Command, []string) error {
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
		Label: "orderID",
		Validate: func(input string) error {
			if len(input) == 0 {
				return errors.New("input is empty")
			}
			_, err := ids.FromString(input)
			return err
		},
	}
	rawOrderID, err := promptText.Run()
	if err != nil {
		return err
	}
	orderID, err := ids.FromString(rawOrderID)
	if err != nil {
		return err
	}

	// Select outbound token
	// TODO: select this automatically
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
	rawAsset, err := promptText.Run()
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

	submit, tx, _, err := cli.GenerateTransaction(ctx, &actions.CloseOrder{
		Order: orderID,
		Out:   outAssetID,
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
