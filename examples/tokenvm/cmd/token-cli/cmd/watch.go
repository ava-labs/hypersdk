// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
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
					summaryStr = fmt.Sprintf("%d %s -> %s", action.Value, action.Asset, tutils.Address(action.To))
				case *actions.BurnAsset:
					summaryStr = fmt.Sprintf("%d %s -> 🔥", action.Value, action.Asset)
				case *actions.ModifyAsset:
					summaryStr = fmt.Sprintf("assetID: %s metadata:%s owner:%s", action.Asset, string(action.Metadata), tutils.Address(action.Owner))

				case *actions.Transfer:
					summaryStr = fmt.Sprintf("%d %s -> %s", action.Value, action.Asset, tutils.Address(action.To))

				case *actions.CreateOrder:
					summaryStr = fmt.Sprintf("%d %s -> %d %s (supply: %d)", action.InTick, action.In, action.OutTick, action.Out, action.Supply)
				case *actions.FillOrder:
					or, _ := actions.UnmarshalOrderResult(result.Output)
					summaryStr = fmt.Sprintf("%d %s -> %d %s (remaining: %d)", or.In, action.In, or.Out, action.Out, or.Remaining)
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
