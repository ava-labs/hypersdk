// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package throughput implements the SpamHelper interface. This package is not
// required to be implemented by the VM developer.

package throughput

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/testvm/actions"
	"github.com/ava-labs/hypersdk/examples/testvm/vm"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/throughput"
	"github.com/ava-labs/hypersdk/utils"

	mauth "github.com/ava-labs/hypersdk/examples/testvm/auth"
)

type SpamHelper struct {
	InitialBalance uint64
	KeyType string
	cli     *vm.JSONRPCClient
	ws      *ws.WebSocketClient
}

var _ throughput.SpamHelper = &SpamHelper{}

func (sh *SpamHelper) CreateAccount() (*auth.PrivateKey, error) {
	return mauth.GeneratePrivateKey(sh.KeyType)
}

func (sh *SpamHelper) CreateClient(uri string) error {
	sh.cli = vm.NewJSONRPCClient(uri)
	ws, err := ws.NewWebSocketClient(uri, ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		return err
	}
	sh.ws = ws
	return nil
}

func (sh *SpamHelper) GetParser(ctx context.Context) (chain.Parser, error) {
	return sh.cli.Parser(ctx)
}

func (sh *SpamHelper) LookupBalance(address codec.Address) (uint64, error) {
	return sh.InitialBalance, nil
}

func (*SpamHelper) GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action {
	return []chain.Action{&actions.Transfer{
		To:    address,
		Value: amount,
		Memo:  memo,
	}}
}

func (*SpamHelper) GetAction() []chain.Action {
	utils.Outf("{{magenta}} creating an action \n}}")
	id := ids.GenerateTestID()
	address := codec.CreateAddress(0, id)
	return []chain.Action{&actions.Count{
		Address: address,
		Amount:  1,
	}}
}

