// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	frpc "github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-faucet/rpc"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var fundFaucetCmd = &cobra.Command{
	Use: "fund-faucet",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()

		// Get faucet
		faucetURI, err := handler.Root().PromptString("faucet URI", 0, consts.MaxInt)
		if err != nil {
			return err
		}
		fcli := frpc.NewJSONRPCClient(faucetURI)
		faucetAddress, err := fcli.FaucetAddress(ctx)
		if err != nil {
			return err
		}

		// Get clients
		_, priv, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Get balance
		_, decimals, balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.Address, ids.Empty, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", decimals, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		addr, err := codec.ParseAddressBech32(tconsts.HRP, faucetAddress)
		if err != nil {
			return err
		}
		if err = sendAndWait(ctx, &actions.Transfer{
			To:    addr,
			Asset: ids.Empty,
			Value: amount,
		}, cli, scli, tcli, factory, true); err != nil {
			return err
		}
		hutils.Outf("{{green}}funded faucet:{{/}} %s\n", faucetAddress)
		return nil
	},
}

var transferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select token to send
		assetID, err := handler.Root().PromptAsset("assetID", true)
		if err != nil {
			return err
		}
		_, decimals, balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.Address, assetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", decimals, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		err = sendAndWait(ctx, &actions.Transfer{
			To:    recipient,
			Asset: assetID,
			Value: amount,
		}, cli, scli, tcli, factory, true)
		return err
	},
}

var createAssetCmd = &cobra.Command{
	Use: "create-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Add symbol to token
		symbol, err := handler.Root().PromptString("symbol", 1, actions.MaxSymbolSize)
		if err != nil {
			return err
		}

		// Add decimal to token
		decimals, err := handler.Root().PromptInt("decimals", actions.MaxDecimals)
		if err != nil {
			return err
		}

		// Add metadata to token
		metadata, err := handler.Root().PromptString("metadata", 1, actions.MaxMetadataSize)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		err = sendAndWait(ctx, &actions.CreateAsset{
			Symbol:   []byte(symbol),
			Decimals: uint8(decimals), // already constrain above to prevent overflow
			Metadata: []byte(metadata),
		}, cli, scli, tcli, factory, true)
		return err
	},
}

var mintAssetCmd = &cobra.Command{
	Use: "mint-asset",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select token to mint
		assetID, err := handler.Root().PromptAsset("assetID", false)
		if err != nil {
			return err
		}
		exists, symbol, decimals, metadata, supply, owner, err := tcli.Asset(ctx, assetID, false)
		if err != nil {
			return err
		}
		if !exists {
			hutils.Outf("{{red}}%s does not exist{{/}}\n", assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		if owner != codec.MustAddressBech32(tconsts.HRP, priv.Address) {
			hutils.Outf("{{red}}%s is the owner of %s, you are not{{/}}\n", owner, assetID)
			hutils.Outf("{{red}}exiting...{{/}}\n")
			return nil
		}
		hutils.Outf(
			"{{yellow}}symbol:{{/}} %s {{yellow}}decimals:{{/}} %s {{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
			string(symbol),
			decimals,
			string(metadata),
			supply,
		)

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", decimals, consts.MaxUint64-supply, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		err = sendAndWait(ctx, &actions.MintAsset{
			Asset: assetID,
			To:    recipient,
			Value: amount,
		}, cli, scli, tcli, factory, true)
		return err
	},
}

var closeOrderCmd = &cobra.Command{
	Use: "close-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		orderID, err := handler.Root().PromptID("orderID")
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		err = sendAndWait(ctx, &actions.CloseOrder{
			Order: orderID,
			Out:   outAssetID,
		}, cli, scli, tcli, factory, true)
		return err
	},
}

var createOrderCmd = &cobra.Command{
	Use: "create-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		inAssetID, err := handler.Root().PromptAsset("in assetID", true)
		if err != nil {
			return err
		}
		exists, symbol, decimals, metadata, supply, _, err := tcli.Asset(ctx, inAssetID, false)
		if err != nil {
			return err
		}
		if inAssetID != ids.Empty {
			if !exists {
				hutils.Outf("{{red}}%s does not exist{{/}}\n", inAssetID)
				hutils.Outf("{{red}}exiting...{{/}}\n")
				return nil
			}
			hutils.Outf(
				"{{yellow}}symbol:{{/}} %s {{yellow}}decimals:{{/}} %d {{yellow}}metadata:{{/}} %s {{yellow}}supply:{{/}} %d\n",
				string(symbol),
				decimals,
				string(metadata),
				supply,
			)
		}

		// Select in tick
		inTick, err := handler.Root().PromptAmount("in tick", decimals, consts.MaxUint64, nil)
		if err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}
		_, decimals, balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.Address, outAssetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select out tick
		outTick, err := handler.Root().PromptAmount("out tick", decimals, consts.MaxUint64, nil)
		if err != nil {
			return err
		}

		// Select supply
		supply, err = handler.Root().PromptAmount(
			"supply (must be multiple of out tick)",
			decimals,
			balance,
			func(input uint64) error {
				if input%outTick != 0 {
					return ErrNotMultiple
				}
				return nil
			},
		)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		err = sendAndWait(ctx, &actions.CreateOrder{
			In:      inAssetID,
			InTick:  inTick,
			Out:     outAssetID,
			OutTick: outTick,
			Supply:  supply,
		}, cli, scli, tcli, factory, true)
		return err
	},
}

var fillOrderCmd = &cobra.Command{
	Use: "fill-order",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, scli, tcli, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Select inbound token
		inAssetID, err := handler.Root().PromptAsset("in assetID", true)
		if err != nil {
			return err
		}
		inSymbol, inDecimals, balance, _, err := handler.GetAssetInfo(ctx, tcli, priv.Address, inAssetID, true)
		if balance == 0 || err != nil {
			return err
		}

		// Select outbound token
		outAssetID, err := handler.Root().PromptAsset("out assetID", true)
		if err != nil {
			return err
		}
		outSymbol, outDecimals, _, _, err := handler.GetAssetInfo(ctx, tcli, priv.Address, outAssetID, false)
		if err != nil {
			return err
		}

		// View orders
		orders, err := tcli.Orders(ctx, actions.PairID(inAssetID, outAssetID))
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
			hutils.Outf(
				"%d) {{cyan}}Rate(in/out):{{/}} %.4f {{cyan}}InTick:{{/}} %s %s {{cyan}}OutTick:{{/}} %s %s {{cyan}}Remaining:{{/}} %s %s\n", //nolint:lll
				i,
				float64(order.InTick)/float64(order.OutTick),
				hutils.FormatBalance(order.InTick, inDecimals),
				inSymbol,
				hutils.FormatBalance(order.OutTick, outDecimals),
				outSymbol,
				hutils.FormatBalance(order.Remaining, outDecimals),
				outSymbol,
			)
		}

		// Select order
		orderIndex, err := handler.Root().PromptChoice("select order", max)
		if err != nil {
			return err
		}
		order := orders[orderIndex]

		// Select input to trade
		value, err := handler.Root().PromptAmount(
			"value (must be multiple of in tick)",
			inDecimals,
			balance,
			func(input uint64) error {
				if input%order.InTick != 0 {
					return ErrNotMultiple
				}
				multiples := input / order.InTick
				requiredRemainder := order.OutTick * multiples
				if requiredRemainder > order.Remaining {
					return ErrInsufficientSupply
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
		multiples := value / order.InTick
		outAmount := multiples * order.OutTick
		hutils.Outf(
			"{{orange}}in:{{/}} %s %s {{orange}}out:{{/}} %s %s\n",
			hutils.FormatBalance(value, inDecimals),
			inSymbol,
			hutils.FormatBalance(outAmount, outDecimals),
			outSymbol,
		)

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		owner, err := codec.ParseAddressBech32(tconsts.HRP, order.Owner)
		if err != nil {
			return err
		}
		err = sendAndWait(ctx, &actions.FillOrder{
			Order: order.ID,
			Owner: owner,
			In:    inAssetID,
			Out:   outAssetID,
			Value: value,
		}, cli, scli, tcli, factory, true)
		return err
	},
}
