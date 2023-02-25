// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	tutils "github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [options]",
	Short: "Watch monitors network activity",
	RunE:  watchFunc,
}

func watchFunc(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
	cli := client.New(uri)
	port, err := cli.BlocksPort(ctx)
	if err != nil {
		return err
	}
	host, err := utils.GetHost(uri)
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
	utils.Outf("{{green}}watching for new blocks 👀{{/}}\n")
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
}
