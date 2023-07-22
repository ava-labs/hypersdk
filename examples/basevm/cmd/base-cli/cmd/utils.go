package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	brpc "github.com/ava-labs/hypersdk/examples/basevm/rpc"
	"github.com/ava-labs/hypersdk/rpc"
)

// TODO: use websockets
func sendAndWait(
	ctx context.Context, warpMsg *warp.Message, action chain.Action, cli *rpc.JSONRPCClient,
	bcli *brpc.JSONRPCClient, factory chain.AuthFactory, printStatus bool,
) (bool, ids.ID, error) {
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
	success, err := bcli.WaitForTransaction(ctx, tx.ID())
	if err != nil {
		return false, ids.Empty, err
	}
	if printStatus {
		handler.Root().PrintStatus(tx.ID(), success)
	}
	return success, tx.ID(), nil
}
