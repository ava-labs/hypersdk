// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:lll
package cmd

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

var chainCmd = &cobra.Command{
	Use: "chain",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var importChainCmd = &cobra.Command{
	Use: "import",
	RunE: func(_ *cobra.Command, args []string) error {
		chainID, err := promptID("chainID")
		if err != nil {
			return err
		}
		uri, err := promptString("uri")
		if err != nil {
			return err
		}
		if err := StoreChain(chainID, uri); err != nil {
			return err
		}
		if err := StoreDefault(defaultChainKey, chainID[:]); err != nil {
			return err
		}
		return nil
	},
}

var setChainCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		chainID, _, err := promptChain("set default chain", nil)
		if err != nil {
			return err
		}
		return StoreDefault(defaultChainKey, chainID[:])
	},
}

var chainInfoCmd = &cobra.Command{
	Use: "info",
	RunE: func(_ *cobra.Command, args []string) error {
		_, uris, err := promptChain("select chainID", nil)
		if err != nil {
			return err
		}
		cli := client.New(uris[0])
		networkID, subnetID, chainID, err := cli.Network(context.Background())
		if err != nil {
			return err
		}
		utils.Outf(
			"{{cyan}}networkID:{{/}} %d {{cyan}}subnetID:{{/}} %s {{cyan}}chainID:{{/}} %s",
			networkID,
			subnetID,
			chainID,
		)
		return nil
	},
}

var watchChainCmd = &cobra.Command{
	Use: "watch",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		chainID, uris, err := promptChain("select chainID", nil)
		if err != nil {
			return err
		}
		if err := CloseDatabase(); err != nil {
			return err
		}
		cli := client.New(uris[0])
		port, err := cli.BlocksPort(ctx)
		if err != nil {
			return err
		}
		host, err := utils.GetHost(uris[0])
		if err != nil {
			return err
		}
		scli, err := vm.NewBlockRPCClient(fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return err
		}
		defer scli.Close()
		parser, err := cli.Parser(ctx)
		if err != nil {
			return err
		}
		totalTxs := float64(0)
		start := time.Now()
		utils.Outf("{{green}}watching for new blocks on %s 👀{{/}}\n", chainID)
		for ctx.Err() == nil {
			blk, results, err := scli.Listen(parser)
			if err != nil {
				return err
			}
			totalTxs += float64(len(blk.Txs))
			utils.Outf(
				"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}units:{{/}}%d {{green}}root:{{/}}%s {{green}}avg TPS:{{/}}%f\n", //nolint:lll
				blk.Hght,
				len(blk.Txs),
				blk.UnitsConsumed,
				blk.StateRoot,
				totalTxs/time.Since(start).Seconds(),
			)
			for i, tx := range blk.Txs {
				result := results[i]
				summaryStr := string(result.Output)
				actor := auth.GetActor(tx.Auth)
				status := "⚠️"
				if result.Success {
					status = "✅"
					switch action := tx.Action.(type) {
					case *actions.CreateAsset:
						summaryStr = fmt.Sprintf("assetID: %s metadata:%s", tx.ID(), string(action.Metadata))
					case *actions.MintAsset:
						amountStr := strconv.FormatUint(action.Value, 10)
						assetStr := action.Asset.String()
						if action.Asset == ids.Empty {
							amountStr = utils.FormatBalance(action.Value)
							assetStr = consts.Symbol
						}
						summaryStr = fmt.Sprintf("%s %s -> %s", amountStr, assetStr, tutils.Address(action.To))
					case *actions.BurnAsset:
						summaryStr = fmt.Sprintf("%d %s -> 🔥", action.Value, action.Asset)
					case *actions.ModifyAsset:
						summaryStr = fmt.Sprintf(
							"assetID: %s metadata:%s owner:%s",
							action.Asset, string(action.Metadata), tutils.Address(action.Owner),
						)

					case *actions.Transfer:
						amountStr := strconv.FormatUint(action.Value, 10)
						assetStr := action.Asset.String()
						if action.Asset == ids.Empty {
							amountStr = utils.FormatBalance(action.Value)
							assetStr = consts.Symbol
						}
						summaryStr = fmt.Sprintf("%s %s -> %s", amountStr, assetStr, tutils.Address(action.To))

					case *actions.CreateOrder:
						inTickStr := strconv.FormatUint(action.InTick, 10)
						inStr := action.In.String()
						if action.In == ids.Empty {
							inTickStr = utils.FormatBalance(action.InTick)
							inStr = consts.Symbol
						}
						outTickStr := strconv.FormatUint(action.OutTick, 10)
						supplyStr := strconv.FormatUint(action.Supply, 10)
						outStr := action.Out.String()
						if action.Out == ids.Empty {
							outTickStr = utils.FormatBalance(action.OutTick)
							supplyStr = utils.FormatBalance(action.Supply)
							outStr = consts.Symbol
						}
						summaryStr = fmt.Sprintf("%s %s -> %s %s (supply: %s %s)", inTickStr, inStr, outTickStr, outStr, supplyStr, outStr)
					case *actions.FillOrder:
						or, _ := actions.UnmarshalOrderResult(result.Output)
						inAmtStr := strconv.FormatUint(or.In, 10)
						inStr := action.In.String()
						if action.In == ids.Empty {
							inAmtStr = utils.FormatBalance(or.In)
							inStr = consts.Symbol
						}
						outAmtStr := strconv.FormatUint(or.Out, 10)
						remainingStr := strconv.FormatUint(or.Remaining, 10)
						outStr := action.Out.String()
						if action.Out == ids.Empty {
							outAmtStr = utils.FormatBalance(or.Out)
							remainingStr = utils.FormatBalance(or.Remaining)
							outStr = consts.Symbol
						}
						summaryStr = fmt.Sprintf(
							"%s %s -> %s %s (remaining: %s %s)",
							inAmtStr, inStr, outAmtStr, outStr, remainingStr, outStr,
						)
					case *actions.CloseOrder:
						summaryStr = fmt.Sprintf("orderID: %s", action.Order)

					case *actions.ImportAsset:
						wm := tx.WarpMessage
						signers, _ := wm.Signature.NumSigners()
						wt, _ := actions.UnmarshalWarpTransfer(wm.Payload)
						summaryStr = fmt.Sprintf("source: %s signers: %d | ", wm.SourceChainID, signers)
						var outputAssetID ids.ID
						if wt.Return {
							outputAssetID = wt.Asset
							summaryStr += fmt.Sprintf("%s %s -> %s (return: %t)", valueString(wt.Asset, wt.Value), assetString(wt.Asset), tutils.Address(wt.To), wt.Return)
						} else {
							outputAssetID = actions.ImportedAssetID(wt.Asset, wm.SourceChainID)
							summaryStr += fmt.Sprintf("%s %s (original: %s) -> %s (return: %t)", valueString(outputAssetID, wt.Value), outputAssetID, wt.Asset, tutils.Address(wt.To), wt.Return)
						}
						if wt.Reward > 0 {
							summaryStr += fmt.Sprintf(" | reward: %s", valueString(outputAssetID, wt.Reward))
						}
						if wt.SwapIn > 0 {
							summaryStr += fmt.Sprintf(" | swap in: %s %s swap out: %s %s expiry: %d fill: %t", valueString(outputAssetID, wt.SwapIn), assetString(outputAssetID), valueString(wt.AssetOut, wt.SwapOut), assetString(wt.AssetOut), wt.SwapExpiry, action.Fill)
						}
					case *actions.ExportAsset:
						wt, _ := actions.UnmarshalWarpTransfer(result.WarpMessage.Payload)
						summaryStr = fmt.Sprintf("destination: %s | ", action.Destination)
						var outputAssetID ids.ID
						if !action.Return {
							outputAssetID = actions.ImportedAssetID(action.Asset, result.WarpMessage.SourceChainID)
							summaryStr += fmt.Sprintf("%s %s -> %s (return: %t)", valueString(action.Asset, action.Value), assetString(action.Asset), tutils.Address(action.To), action.Return)
						} else {
							outputAssetID = wt.Asset
							summaryStr += fmt.Sprintf("%s %s (original: %s) -> %s (return: %t)", valueString(action.Asset, action.Value), action.Asset, assetString(wt.Asset), tutils.Address(action.To), action.Return)
						}
						if wt.Reward > 0 {
							summaryStr += fmt.Sprintf(" | reward: %s", valueString(outputAssetID, wt.Reward))
						}
						if wt.SwapIn > 0 {
							summaryStr += fmt.Sprintf(" | swap in: %s %s swap out: %s %s expiry: %d", valueString(outputAssetID, wt.SwapIn), assetString(outputAssetID), valueString(wt.AssetOut, wt.SwapOut), assetString(wt.AssetOut), wt.SwapExpiry)
						}
					}
				}
				utils.Outf(
					"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}units:{{/}} %d {{yellow}}summary (%s):{{/}} [%s]\n",
					status,
					tx.ID(),
					tutils.Address(actor),
					result.Units,
					reflect.TypeOf(tx.Action),
					summaryStr,
				)
			}
		}
		return nil
	},
}
