// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run",
	RunE: func(*cobra.Command, []string) error {
		var sclient *rpc.WebSocketClient
		var tclient *trpc.JSONRPCClient
		var maxFeeParsed *uint64
		if maxFee >= 0 {
			v := uint64(maxFee)
			maxFeeParsed = &v
		}
		return handler.Root().Spam(maxTxBacklog, maxFeeParsed, randomRecipient,
			func(uri string, networkID uint32, chainID ids.ID) {
				tclient = trpc.NewJSONRPCClient(uri, networkID, chainID)
				sc, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
				if err != nil {
					panic(err)
				}
				sclient = sc
			},
			func(pk ed25519.PrivateKey) chain.AuthFactory {
				return auth.NewED25519Factory(pk)
			},
			func(choice int, address string) (uint64, error) {
				balance, err := tclient.Balance(context.TODO(), address, ids.Empty)
				if err != nil {
					return 0, err
				}
				utils.Outf(
					"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
					choice,
					address,
					utils.FormatBalance(balance, consts.Decimals),
					consts.Symbol,
				)
				return balance, err
			},
			func(ctx context.Context, chainID ids.ID) (chain.Parser, error) {
				return tclient.Parser(ctx)
			},
			func(pk ed25519.PublicKey, amount uint64) chain.Action {
				return &actions.Transfer{
					To:    pk,
					Asset: ids.Empty,
					Value: amount,
				}
			},
			func(cli *rpc.JSONRPCClient, pk ed25519.PrivateKey) func(context.Context, uint64) error {
				return func(ictx context.Context, count uint64) error {
					_, _, err := sendAndWait(ictx, nil, &actions.Transfer{
						To:    pk.PublicKey(),
						Value: count, // prevent duplicate txs
					}, cli, sclient, tclient, auth.NewED25519Factory(pk), false)
					return err
				}
			},
		)
	},
}
