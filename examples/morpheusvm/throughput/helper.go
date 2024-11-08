// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package throughput implements the SpamHelper interface. This package is not
// required to be implemented by the VM developer.

package throughput

import (
	"context"
	"encoding/binary"
	"sync/atomic"

	"golang.org/x/exp/rand"

	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/throughput"
)

type SpamHelper struct {
	KeyType string
	cli     *vm.JSONRPCClient
	ws      *ws.WebSocketClient

	pks  []*auth.PrivateKey
	sent atomic.Int64
}

var _ throughput.SpamHelper = &SpamHelper{}

func (sh *SpamHelper) CreateAccount() (*auth.PrivateKey, error) {
	pk, err := vm.AuthProvider.GeneratePrivateKey(sh.KeyType)
	if err != nil {
		return nil, err
	}

	sh.pks = append(sh.pks, pk)
	return pk, nil
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
	balance, err := sh.cli.Balance(context.TODO(), address)
	if err != nil {
		return 0, err
	}

	return balance, err
}

func (*SpamHelper) GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action {
	return []chain.Action{&actions.Transfer{
		To:    address,
		Value: amount,
		Memo:  memo,
	}}
}

func (sh *SpamHelper) GetActions() []chain.Action {
	pkIndex := rand.Int() % len(sh.pks)
	// transfers 1 unit to a random address
	return sh.GetTransfer(sh.pks[pkIndex].Address, 1, sh.uniqueBytes())
}

func (sh *SpamHelper) uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sh.sent.Add(1)))
}
