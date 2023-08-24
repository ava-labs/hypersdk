// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	tutils "github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

// TODO: use websockets
func sendAndWait(
	ctx context.Context, warpMsg *warp.Message, action chain.Action, cli *rpc.JSONRPCClient,
	bcli *brpc.JSONRPCClient, factory chain.AuthFactory, printStatus bool,
) (bool, ids.ID, error) { //nolint:unparam
	parser, err := bcli.Parser(ctx)
	if err != nil {
		return false, ids.Empty, err
	}
	submit, tx, _, err := cli.GenerateTransaction(ctx, parser, warpMsg, action, factory)
	if err != nil {
		return false, ids.Empty, err
	}
	if err := submit(ctx); err != nil {
		return false, ids.Empty, err
	}
	success, _, err := bcli.WaitForTransaction(ctx, tx.ID())
	if err != nil {
		return false, ids.Empty, err
	}
	if printStatus {
		handler.Root().PrintStatus(tx.ID(), success)
	}
	return success, tx.ID(), nil
}

func handleTx(tx *chain.Transaction, result *chain.Result) {
	summaryStr := string(result.Output)
	actor := auth.GetActor(tx.Auth)
	status := "⚠️"
	if result.Success {
		status = "✅"
		switch action := tx.Action.(type) { //nolint:gocritic
		case *actions.Transfer:
			summaryStr = fmt.Sprintf("%s %s -> %s", utils.FormatBalance(action.Value), consts.Symbol, tutils.Address(action.To))
		}
	}
	utils.Outf(
		"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}summary (%s):{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
		status,
		tx.ID(),
		tutils.Address(actor),
		reflect.TypeOf(tx.Action),
		summaryStr,
		float64(result.Fee)/float64(tx.Base.MaxFee)*100,
		handler.Root().ValueString(ids.Empty, result.Fee),
		handler.Root().AssetString(ids.Empty),
		cli.ParseDimensions(result.Consumed),
	)
}
