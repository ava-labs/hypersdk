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
	"github.com/ava-labs/avalanchego/utils/math"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/window"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
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
		return handler.Root().ImportChain()
	},
}

var importANRChainCmd = &cobra.Command{
	Use: "import-anr",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportANR()
	},
}

var importAvalancheOpsChainCmd = &cobra.Command{
	Use: "import-ops [chainID] [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return ErrInvalidArgs
		}
		_, err := ids.FromString(args[0])
		return err
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportOps(args[0], args[1])
	},
}

var setChainCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().SetDefaultChain()
	},
}

var chainInfoCmd = &cobra.Command{
	Use: "info",
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().PrintChainInfo()
	},
}

var watchChainCmd = &cobra.Command{
	Use: "watch",
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		chainID, uris, err := handler.Root().PromptChain("select chainID", nil)
		if err != nil {
			return err
		}
		if err := handler.Root().CloseDatabase(); err != nil {
			return err
		}
		rcli := rpc.NewJSONRPCClient(uris[0])
		networkID, _, _, err := rcli.Network(context.TODO())
		if err != nil {
			return err
		}
		cli := trpc.NewJSONRPCClient(uris[0], networkID, chainID)
		utils.Outf("{{yellow}}uri:{{/}} %s\n", uris[0])
		scli, err := rpc.NewWebSocketClient(uris[0])
		if err != nil {
			return err
		}
		defer scli.Close()
		if err := scli.RegisterBlocks(); err != nil {
			return err
		}
		parser, err := cli.Parser(ctx)
		if err != nil {
			return err
		}
		utils.Outf("{{green}}watching for new blocks on %s 👀{{/}}\n", chainID)
		var (
			start             time.Time
			lastBlock         int64
			lastBlockDetailed time.Time
			tpsWindow         = window.Window{}
		)
		for ctx.Err() == nil {
			blk, results, err := scli.ListenBlock(ctx, parser)
			if err != nil {
				return err
			}
			now := time.Now()
			if start.IsZero() {
				start = now
			}
			if lastBlock != 0 {
				since := now.Unix() - lastBlock
				newWindow, err := window.Roll(tpsWindow, int(since))
				if err != nil {
					return err
				}
				tpsWindow = newWindow
				window.Update(&tpsWindow, window.WindowSliceSize-hconsts.Uint64Len, uint64(len(blk.Txs)))
				runningDuration := time.Since(start)
				tpsDivisor := math.Min(window.WindowSize, runningDuration.Seconds())
				utils.Outf(
					"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}units:{{/}}%d {{green}}root:{{/}}%s {{green}}TPS:{{/}}%.2f {{green}}split:{{/}}%dms\n", //nolint:lll
					blk.Hght,
					len(blk.Txs),
					blk.UnitsConsumed,
					blk.StateRoot,
					float64(window.Sum(tpsWindow))/tpsDivisor,
					time.Since(lastBlockDetailed).Milliseconds(),
				)
			} else {
				utils.Outf(
					"{{green}}height:{{/}}%d {{green}}txs:{{/}}%d {{green}}units:{{/}}%d {{green}}root:{{/}}%s\n", //nolint:lll
					blk.Hght,
					len(blk.Txs),
					blk.UnitsConsumed,
					blk.StateRoot,
				)
				window.Update(&tpsWindow, window.WindowSliceSize-hconsts.Uint64Len, uint64(len(blk.Txs)))
			}
			lastBlock = now.Unix()
			lastBlockDetailed = now
			if hideTxs {
				continue
			}
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
							summaryStr += fmt.Sprintf("%s %s -> %s (return: %t)", handler.Root().ValueString(wt.Asset, wt.Value), handler.Root().AssetString(wt.Asset), tutils.Address(wt.To), wt.Return)
						} else {
							outputAssetID = actions.ImportedAssetID(wt.Asset, wm.SourceChainID)
							summaryStr += fmt.Sprintf("%s %s (original: %s) -> %s (return: %t)", handler.Root().ValueString(outputAssetID, wt.Value), outputAssetID, wt.Asset, tutils.Address(wt.To), wt.Return)
						}
						if wt.Reward > 0 {
							summaryStr += fmt.Sprintf(" | reward: %s", handler.Root().ValueString(outputAssetID, wt.Reward))
						}
						if wt.SwapIn > 0 {
							summaryStr += fmt.Sprintf(" | swap in: %s %s swap out: %s %s expiry: %d fill: %t", handler.Root().ValueString(outputAssetID, wt.SwapIn), handler.Root().AssetString(outputAssetID), handler.Root().ValueString(wt.AssetOut, wt.SwapOut), handler.Root().AssetString(wt.AssetOut), wt.SwapExpiry, action.Fill)
						}
					case *actions.ExportAsset:
						wt, _ := actions.UnmarshalWarpTransfer(result.WarpMessage.Payload)
						summaryStr = fmt.Sprintf("destination: %s | ", action.Destination)
						var outputAssetID ids.ID
						if !action.Return {
							outputAssetID = actions.ImportedAssetID(action.Asset, result.WarpMessage.SourceChainID)
							summaryStr += fmt.Sprintf("%s %s -> %s (return: %t)", handler.Root().ValueString(action.Asset, action.Value), handler.Root().AssetString(action.Asset), tutils.Address(action.To), action.Return)
						} else {
							outputAssetID = wt.Asset
							summaryStr += fmt.Sprintf("%s %s (original: %s) -> %s (return: %t)", handler.Root().ValueString(action.Asset, action.Value), action.Asset, handler.Root().AssetString(wt.Asset), tutils.Address(action.To), action.Return)
						}
						if wt.Reward > 0 {
							summaryStr += fmt.Sprintf(" | reward: %s", handler.Root().ValueString(outputAssetID, wt.Reward))
						}
						if wt.SwapIn > 0 {
							summaryStr += fmt.Sprintf(" | swap in: %s %s swap out: %s %s expiry: %d", handler.Root().ValueString(outputAssetID, wt.SwapIn), handler.Root().AssetString(outputAssetID), handler.Root().ValueString(wt.AssetOut, wt.SwapOut), handler.Root().AssetString(wt.AssetOut), wt.SwapExpiry)
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
