// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"golang.org/x/exp/rand"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/examples/hyperevm/vm"
	"github.com/ava-labs/hypersdk/throughput"
)

var _ throughput.SpamHelper = &SpamHelper{}

type SpamHelper struct {
	KeyType string
	cli     *vm.JSONRPCClient

	pks  []*auth.PrivateKey
	sent atomic.Int64
}

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
	return nil
}

func (sh *SpamHelper) GetActions(factory chain.AuthFactory) []chain.Action {
	pkIndex := rand.Int() % len(sh.pks)
	// transfers 1 unit to a random address
	return sh.GetTransfer(sh.pks[pkIndex].Address, 1, sh.uniqueBytes(), factory)
}

func (sh *SpamHelper) GetParser(ctx context.Context) (chain.Parser, error) {
	return sh.cli.Parser(ctx)
}

func (sh *SpamHelper) GetTransfer(address codec.Address, amount uint64, memo []byte, fromFactory chain.AuthFactory) []chain.Action {
	to := storage.ToEVMAddress(address)
	from := storage.ToEVMAddress(fromFactory.Address())

	call := &actions.EvmCall{
		To:       to,
		From:     from,
		Value:    amount,
		GasLimit: 21_000,
		Data:     []byte{},
	}

	simRes, err := sh.cli.SimulateActions(context.TODO(), []chain.Action{call}, fromFactory.Address())
	if err != nil {
		fmt.Println("simulate actions error", err)
		return []chain.Action{} // TODO: handle this better, return err
	}
	actionResult := simRes[0]
	call.Keys = actionResult.StateKeys

	return []chain.Action{call}
}

func (sh *SpamHelper) LookupBalance(address codec.Address) (uint64, error) {
	balance, err := sh.cli.Balance(context.TODO(), address)
	if err != nil {
		return 0, err
	}

	return balance, err
}

func (sh *SpamHelper) uniqueBytes() []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(sh.sent.Add(1)))
}
