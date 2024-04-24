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
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

// sendAndWait may not be used concurrently
func sendAndWait(
	ctx context.Context, action chain.Action, cli *rpc.JSONRPCClient,
	bcli *brpc.JSONRPCClient, ws *rpc.WebSocketClient, factory chain.AuthFactory, printStatus bool,
) (bool, ids.ID, error) { //nolint:unparam
	parser, err := bcli.Parser(ctx)
	if err != nil {
		return false, ids.Empty, err
	}
	_, tx, _, err := cli.GenerateTransaction(ctx, parser, action, factory)
	if err != nil {
		return false, ids.Empty, err
	}
	if err := ws.RegisterTx(tx); err != nil {
		return false, ids.Empty, err
	}
	var result *chain.Result
	for {
		txID, txErr, txResult, err := ws.ListenTx(ctx)
		if err != nil {
			return false, ids.Empty, err
		}
		if txErr != nil {
			return false, ids.Empty, txErr
		}
		if txID == tx.ID() {
			result = txResult
			break
		}
		utils.Outf("{{yellow}}skipping unexpected transaction:{{/}} %s\n", tx.ID())
	}
	if printStatus {
		handler.Root().PrintStatus(tx.ID(), result.Success)
	}
	return result.Success, tx.ID(), nil
}

func handleTx(tx *chain.Transaction, result *chain.Result) {
	summaryStr := string(result.Output)
	actor := tx.Auth.Actor()
	status := "❌"
	if result.Success {
		status = "✅"
		switch action := tx.Action.(type) { //nolint:gocritic
		case *actions.Transfer:
			summaryStr = fmt.Sprintf("%s %s -> %s", utils.FormatBalance(action.Value, consts.Decimals), consts.Symbol, codec.MustAddressBech32(consts.HRP, action.To))
		}
	}
	utils.Outf(
		"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}summary (%s):{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
		status,
		tx.ID(),
		codec.MustAddressBech32(consts.HRP, actor),
		reflect.TypeOf(tx.Action),
		summaryStr,
		float64(result.Fee)/float64(tx.Base.MaxFee)*100,
		utils.FormatBalance(result.Fee, consts.Decimals),
		consts.Symbol,
		cli.ParseDimensions(result.Consumed),
	)
}
