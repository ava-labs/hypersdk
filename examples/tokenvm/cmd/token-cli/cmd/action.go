// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var transferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		priv, factory, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		// Select token to send
		assetID, err := promptAsset("assetID", true)
		if err != nil {
			return err
		}
		addr := utils.Address(priv.PublicKey())
		balance, err := cli.Balance(ctx, addr, assetID)
		if err != nil {
			return err
		}
		if balance == 0 {
			hutils.Outf("{{red}}balance:{{/}} 0 %s\n", assetString(assetID))
			hutils.Outf("{{red}}please send funds to %s{{/}}\n", addr)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", valueString(assetID, balance), assetString(assetID))

		// Select recipient
		recipient, err := promptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := promptAmount("amount", assetID, balance, 0)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.Transfer{
			To:    recipient,
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
		printStatus(tx.ID(), success)
		return nil
	},
}

var createAssetCmd = &cobra.Command{
	Use: "create-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, factory, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		// Add metadata to token
		promptText := promptui.Prompt{
			Label: "metadata (can be changed later)",
			Validate: func(input string) error {
				if len(input) > actions.MaxMetadataSize {
					return errors.New("input too large")
				}
				return nil
			},
		}
		metadata, err := promptText.Run()
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.CreateAsset{
			Metadata: []byte(metadata),
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
		printStatus(tx.ID(), success)
		return nil
	},
}

var mintAssetCmd = &cobra.Command{
	Use: "mint-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		priv, factory, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		// Select token to mint
		assetID, err := promptAsset("assetID", false)
		if err != nil {
			return err
		}
		exists, metadata, supply, owner, warp, err := cli.Asset(ctx, assetID)
		if err != nil {
			return err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		if warp {
			hutils.Outf("{{red}}cannot mint a warped asset{{/}}\n", assetID)
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
		recipient, err := promptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := promptAmount("amount", assetID, consts.MaxUint64-supply, 0)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.MintAsset{
			Asset: assetID,
			To:    recipient,
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
		printStatus(tx.ID(), success)
		return nil
	},
}

var closeOrderCmd = &cobra.Command{
	Use: "close-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, factory, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		// Select inbound token
		orderID, err := promptID("orderID")
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := promptAsset("out assetID", true)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.CloseOrder{
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
		printStatus(tx.ID(), success)
		return nil
	},
}

var createOrderCmd = &cobra.Command{
	Use: "create-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		priv, factory, cli, ok, err := defaultActor()
		if !ok || err != nil {
			return err
		}

		// Select inbound token
		inAssetID, err := promptAsset("in assetID", true)
		if err != nil {
			return err
		}
		if inAssetID != ids.Empty {
			exists, metadata, supply, _, warp, err := cli.Asset(ctx, inAssetID)
			if err != nil {
				return err
			}
			if !exists {
				hutils.Outf("{{red}}%s does not exist{{/}}\n", inAssetID)
				hutils.Outf("{{red}}exiting...{{/}}\n")
				return nil
			}
			hutils.Outf(
				"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d {{yellow}}warp:{{/}} %t\n",
				string(metadata),
				supply,
				warp,
			)
		}

		// Select in tick
		inTick, err := promptAmount("in tick", inAssetID, consts.MaxUint64, 0)
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := promptAsset("out assetID", true)
		if err != nil {
			return err
		}
		if outAssetID != ids.Empty {
			exists, metadata, supply, _, warp, err := cli.Asset(ctx, outAssetID)
			if err != nil {
				return err
			}
			if !exists {
				hutils.Outf("{{red}}%s does not exist{{/}}\n", outAssetID)
				hutils.Outf("{{red}}exiting...{{/}}\n")
				return nil
			}
			hutils.Outf(
				"{{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d {{yellow}}warp:{{/}} %t\n",
				string(metadata),
				supply,
				warp,
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
		hutils.Outf("{{yellow}}balance:{{/}} %s %s\n", valueString(outAssetID, balance), assetString(outAssetID))

		// Select out tick
		outTick, err := promptAmount("out tick", outAssetID, consts.MaxUint64, 0)
		if err != nil {
			return err
		}

		// Select supply
		supply, err := promptAmount("supply (must be multiple of out tick)", outAssetID, balance, outTick)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := promptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		submit, tx, _, err := cli.GenerateTransaction(ctx, nil, &actions.CreateOrder{
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
		printStatus(tx.ID(), success)
		return nil
	},
}
