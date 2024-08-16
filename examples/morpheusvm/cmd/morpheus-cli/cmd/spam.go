// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"

	mrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
)

type SpamHelper struct {
	keyType string
	cli     *mrpc.JSONRPCClient
	ws      *rpc.WebSocketClient
}

func (sh *SpamHelper) CreateAccount() (*cli.PrivateKey, error) {
	return generatePrivateKey(sh.keyType)
}

func (*SpamHelper) GetFactory(pk *cli.PrivateKey) (chain.AuthFactory, error) {
	switch pk.Address[0] {
	case auth.ED25519ID:
		return auth.NewED25519Factory(ed25519.PrivateKey(pk.Bytes)), nil
	case auth.SECP256R1ID:
		return auth.NewSECP256R1Factory(secp256r1.PrivateKey(pk.Bytes)), nil
	case auth.BLSID:
		p, err := bls.PrivateKeyFromBytes(pk.Bytes)
		if err != nil {
			return nil, err
		}
		return auth.NewBLSFactory(p), nil
	default:
		return nil, ErrInvalidKeyType
	}
}

func (sh *SpamHelper) CreateClient(uri string, _ uint32, _ ids.ID) error {
	sh.cli = mrpc.NewJSONRPCClient(uri)
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
	balance, err := sh.cli.Balance(context.TODO(), address)
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
	Use: "run [ed25519/secp256r1/bls]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return checkKeyType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().Spam(&SpamHelper{keyType: args[0]})
	},
}
