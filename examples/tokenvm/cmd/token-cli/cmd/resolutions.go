// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"

	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
)

// sendAndWait may not be used concurrently
func sendAndWait(
	ctx context.Context, actions []chain.Action, cli *rpc.JSONRPCClient,
	scli *rpc.WebSocketClient, tcli *trpc.JSONRPCClient, factory chain.AuthFactory, printStatus bool,
) (ids.ID, error) {
	parser, err := tcli.Parser(ctx)
	if err != nil {
		return ids.Empty, err
	}
	_, tx, _, err := cli.GenerateTransaction(ctx, parser, actions, factory)
	if err != nil {
		return ids.Empty, err
	}

	if err := scli.RegisterTx(tx); err != nil {
		return ids.Empty, err
	}
	var res *chain.Result
	for {
		txID, dErr, result, err := scli.ListenTx(ctx)
		if dErr != nil {
			return ids.Empty, dErr
		}
		if err != nil {
			return ids.Empty, err
		}
		if txID == tx.ID() {
			res = result
			break
		}
		// TODO: don't drop these results (may be needed by a different connection)
		utils.Outf("{{yellow}}skipping unexpected transaction:{{/}} %s\n", tx.ID())
	}
	if printStatus {
		handler.Root().PrintStatus(tx.ID(), res.Success)
	}
	return tx.ID(), nil
}

func handleTx(c *trpc.JSONRPCClient, tx *chain.Transaction, result *chain.Result) {
	actor := tx.Auth.Actor()
	if !result.Success {
		utils.Outf(
			"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}error:{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
			"❌",
			tx.ID(),
			codec.MustAddressBech32(tconsts.HRP, actor),
			result.Error,
			float64(result.Fee)/float64(tx.Base.MaxFee)*100,
			utils.FormatBalance(result.Fee, tconsts.Decimals),
			tconsts.Symbol,
			cli.ParseDimensions(result.Units),
		)
		return
	}

	for i, act := range tx.Actions {
		actionID := chain.CreateActionID(tx.ID(), uint8(i))
		var summaryStr string
		switch action := act.(type) {
		case *actions.CreateAsset:
			summaryStr = fmt.Sprintf("assetID: %s symbol: %s decimals: %d metadata: %s", actionID, action.Symbol, action.Decimals, action.Metadata)
		case *actions.MintAsset:
			_, symbol, decimals, _, _, _, err := c.Asset(context.TODO(), action.Asset, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			amountStr := utils.FormatBalance(action.Value, decimals)
			summaryStr = fmt.Sprintf("%s %s -> %s", amountStr, symbol, codec.MustAddressBech32(tconsts.HRP, action.To))
		case *actions.BurnAsset:
			summaryStr = fmt.Sprintf("%d %s -> 🔥", action.Value, action.Asset)
		case *actions.Transfer:
			_, symbol, decimals, _, _, _, err := c.Asset(context.TODO(), action.Asset, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			amountStr := utils.FormatBalance(action.Value, decimals)
			summaryStr = fmt.Sprintf("%s %s -> %s", amountStr, symbol, codec.MustAddressBech32(tconsts.HRP, action.To))
			if len(action.Memo) > 0 {
				summaryStr += fmt.Sprintf(" (memo: %s)", action.Memo)
			}
		case *actions.CreateOrder:
			_, inSymbol, inDecimals, _, _, _, err := c.Asset(context.TODO(), action.In, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			inTickStr := utils.FormatBalance(action.InTick, inDecimals)
			_, outSymbol, outDecimals, _, _, _, err := c.Asset(context.TODO(), action.Out, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			outTickStr := utils.FormatBalance(action.OutTick, outDecimals)
			supplyStr := utils.FormatBalance(action.Supply, outDecimals)
			summaryStr = fmt.Sprintf("%s %s -> %s %s (supply: %s %s)", inTickStr, inSymbol, outTickStr, outSymbol, supplyStr, outSymbol)
		case *actions.FillOrder:
			or, _ := actions.UnmarshalOrderResult(result.Outputs[i][0])
			_, inSymbol, inDecimals, _, _, _, err := c.Asset(context.TODO(), action.In, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			inAmtStr := utils.FormatBalance(or.In, inDecimals)
			_, outSymbol, outDecimals, _, _, _, err := c.Asset(context.TODO(), action.Out, true)
			if err != nil {
				utils.Outf("{{red}}could not fetch asset info:{{/}} %v", err)
				return
			}
			outAmtStr := utils.FormatBalance(or.Out, outDecimals)
			remainingStr := utils.FormatBalance(or.Remaining, outDecimals)
			summaryStr = fmt.Sprintf(
				"%s %s -> %s %s (remaining: %s %s)",
				inAmtStr, inSymbol, outAmtStr, outSymbol, remainingStr, outSymbol,
			)
		case *actions.CloseOrder:
			summaryStr = fmt.Sprintf("orderID: %s", action.Order)
		}
		utils.Outf(
			"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}summary (%s):{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
			"✅",
			tx.ID(),
			codec.MustAddressBech32(tconsts.HRP, actor),
			reflect.TypeOf(act),
			summaryStr,
			float64(result.Fee)/float64(tx.Base.MaxFee)*100,
			utils.FormatBalance(result.Fee, tconsts.Decimals),
			tconsts.Symbol,
			cli.ParseDimensions(result.Units),
		)
	}
}
