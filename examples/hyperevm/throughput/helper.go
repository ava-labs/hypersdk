// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package throughput

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/exp/rand"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/examples/hyperevm/vm"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
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

	recorder := tstate.NewRecorder(immutableDB{memdb.New()})
	bh := &storage.BalanceHandler{}
	if err := bh.AddBalance(context.Background(), fromFactory.Address(), recorder, (amount * 10)); err != nil {
		fmt.Println("failed to add balance: %w", err)
		return nil
	}

	currentTime := time.Now().UnixMilli()
	blockCtx := chain.NewBlockCtx(0, currentTime)
	r := genesis.NewDefaultRules()
	r.MaxBlockUnits = fees.Dimensions{1800000, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64, consts.MaxUint64}
	_, err := call.Execute(context.Background(), blockCtx, r, recorder, fromFactory.Address(), ids.Empty)
	if err != nil {
		fmt.Println("failed to simulate action:", err)
		return nil
	}

	call.Keys = recorder.GetStateKeys()
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

var _ state.Immutable = (*immutableDB)(nil)

type immutableDB struct {
	*memdb.Database
}

func (db immutableDB) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return db.Get(key)
}
