// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"

	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
)

type SpamHelper struct {
	cli *trpc.JSONRPCClient
	ws  *rpc.WebSocketClient
}

func (*SpamHelper) CreateAccount() (*cli.PrivateKey, error) {
	p, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &cli.PrivateKey{
		Address: auth.NewED25519Address(p.PublicKey()),
		Bytes:   p[:],
	}, nil
}

func (*SpamHelper) GetFactory(pk *cli.PrivateKey) (chain.AuthFactory, error) {
	return auth.NewED25519Factory(ed25519.PrivateKey(pk.Bytes)), nil
}

func (sh *SpamHelper) CreateClient(uri string, networkID uint32, chainID ids.ID) error {
	sh.cli = trpc.NewJSONRPCClient(uri, networkID, chainID)
	ws, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return err
	}
	sh.ws = ws
	return nil
}

func (sh *SpamHelper) GetParser(ctx context.Context) (chain.Parser, error) {
	return sh.cli.Parser(ctx)
}

func (sh *SpamHelper) LookupBalance(choice int, address string) (uint64, error) {
	balance, err := sh.cli.Balance(context.TODO(), address, ids.Empty)
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
}

func (*SpamHelper) GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action {
	return []chain.Action{&actions.Transfer{
		To:    address,
		Asset: ids.Empty,
		Value: amount,
		Memo:  memo,
	}}
}

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().Spam(&SpamHelper{})
	},
}
