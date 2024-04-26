// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
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
			func(uri string, networkID uint32, chainID ids.ID) error { // createClient
				tclient = trpc.NewJSONRPCClient(uri, networkID, chainID)
				sc, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
				if err != nil {
					return err
				}
				sclient = sc
				return nil
			},
			func(priv *cli.PrivateKey) (chain.AuthFactory, error) { // getFactory
				return auth.NewED25519Factory(ed25519.PrivateKey(priv.Bytes)), nil
			},
			func() (*cli.PrivateKey, error) { // createAccount
				p, err := ed25519.GeneratePrivateKey()
				if err != nil {
					return nil, err
				}
				return &cli.PrivateKey{
					Address: auth.NewED25519Address(p.PublicKey()),
					Bytes:   p[:],
				}, nil
			},
			func(choice int, address string) (uint64, error) { // lookupBalance
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
			func(ctx context.Context, chainID ids.ID) (chain.Parser, error) { // getParser
				return tclient.Parser(ctx)
			},
			func(addr codec.Address, amount uint64) chain.Action { // getTransfer
				return &actions.Transfer{
					To:    addr,
					Asset: ids.Empty,
					Value: amount,
				}
			},
			func(cli *rpc.JSONRPCClient, priv *cli.PrivateKey) func(context.Context, uint64) error { // submitDummy
				return func(ictx context.Context, count uint64) error {
					return sendAndWait(ictx, &actions.Transfer{
						To:    priv.Address,
						Value: count, // prevent duplicate txs
					}, cli, sclient, tclient, auth.NewED25519Factory(ed25519.PrivateKey(priv.Bytes)), false)
				}
			},
		)
	},
}
