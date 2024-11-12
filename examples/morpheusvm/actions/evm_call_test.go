// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestContract(t *testing.T) {
	require := require.New(t)

	data := common.Hex2Bytes("608060405234801561000f575f80fd5b506101438061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea264697066735822122000afd17ac37e0bb2b68b3ac973de3608be934fa6f2b2e31808f1502fc93a2f2d64736f6c63430008180033")

	sufficientGas := uint64(1000000)

	var from codec.Address
	copy(from[20:], []byte("112233"))

	call := &EvmCall{
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}

	rules := genesis.NewDefaultRules()
	time := time.Now().UnixMilli()
	txID := ids.ID{}

	ctx := context.Background()
	tracer, err := trace.New(trace.Config{Enabled: false})
	require.NoError(err)
	statedb, err := merkledb.New(ctx, memdb.New(), getTestMerkleConfig(tracer))
	require.NoError(err)
	mu := state.NewSimpleMutable(statedb)

	err = InitializeAccount(ctx, mu, from, uint64(10000000))

	require.NoError(err)

	{
		result, err := call.Execute(
			ctx, rules, mu, time, from, txID,
		)
		require.Nil(err)
		require.True(result.(*EvmCallResult).Success)

		fmt.Println("usedGas", result.(*EvmCallResult).UsedGas)
		fmt.Println("return data", common.Bytes2Hex(result.(*EvmCallResult).Return))
	}

	contractAddress := crypto.CreateAddress(ToEVMAddress(from), 0)
	data = common.Hex2Bytes("6057361d000000000000000000000000000000000000000000000000000000000000002a")
	call = &EvmCall{
		To:       &contractAddress,
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}
	{
		result, err := call.Execute(
			ctx, rules, mu, time, from, txID,
		)
		require.Nil(err)
		require.True(result.(*EvmCallResult).Success)

		fmt.Println("usedGas", result.(*EvmCallResult).UsedGas)
		fmt.Println("return data", common.Bytes2Hex(result.(*EvmCallResult).Return))
	}

	data = common.Hex2Bytes("2e64cec1")
	call = &EvmCall{
		To:       &contractAddress,
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}
	require.NoError(mu.Commit(ctx))
	mu = state.NewSimpleMutable(statedb)
	{
		result, err := call.Execute(
			ctx, rules, mu, time, from, txID,
		)
		require.Nil(err)
		require.True(result.(*EvmCallResult).Success)

		fmt.Println("usedGas", result.(*EvmCallResult).UsedGas)
		fmt.Println("return data", common.Bytes2Hex(result.(*EvmCallResult).Return))
	}

	require.NoError(mu.Commit(ctx))
	newRoot, err := statedb.GetMerkleRoot(ctx)
	require.NoError(err)
	fmt.Println(newRoot)
}

func TestContractWithTracing(t *testing.T) {
	require := require.New(t)
	data := common.Hex2Bytes("608060405234801561000f575f80fd5b506101438061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea264697066735822122000afd17ac37e0bb2b68b3ac973de3608be934fa6f2b2e31808f1502fc93a2f2d64736f6c63430008180033")

	sufficientGas := uint64(1000000)

	var from codec.Address
	copy(from[20:], []byte("112233"))
	call := &EvmCall{
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}

	r := genesis.NewDefaultRules()
	time := time.Now().UnixMilli()
	txID := ids.ID{}

	ctx := context.Background()
	tracer, err := trace.New(trace.Config{Enabled: false})
	require.NoError(err)
	statedb, err := merkledb.New(ctx, memdb.New(), getTestMerkleConfig(tracer))
	require.NoError(err)

	view := state.View(statedb)
	view = traceAndExecute(ctx, require, call, view, r, time, from, txID, tracer)

	contractAddress := crypto.CreateAddress(ToEVMAddress(from), 0)
	data = common.Hex2Bytes("6057361d000000000000000000000000000000000000000000000000000000000000002a")
	call = &EvmCall{
		To:       &contractAddress,
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}
	view = traceAndExecute(ctx, require, call, view, r, time, from, txID, tracer)

	data = common.Hex2Bytes("2e64cec1")
	call = &EvmCall{
		To:       &contractAddress,
		Value:    common.Big0,
		GasLimit: sufficientGas,
		Data:     data,
	}
	_ = traceAndExecute(ctx, require, call, view, r, time, from, txID, tracer)
}
func getTestMerkleConfig(tracer trace.Tracer) merkledb.Config {
	return merkledb.Config{
		BranchFactor:                merkledb.BranchFactor16,
		RootGenConcurrency:          1,
		HistoryLength:               100,
		ValueNodeCacheSize:          units.MiB,
		IntermediateNodeCacheSize:   units.MiB,
		IntermediateWriteBufferSize: units.KiB,
		IntermediateWriteBatchSize:  units.KiB,
		Tracer:                      tracer,
	}
}

func traceAndExecute(ctx context.Context, require *require.Assertions, call *EvmCall, view state.View, r chain.Rules, time int64, from codec.Address, txID ids.ID, tracer trace.Tracer) state.View {
	mu := state.NewSimpleMutable(view)
	err := InitializeAccount(ctx, mu, from, uint64(10000000))
	require.NoError(err)
	recorder := tstate.NewRecorder(mu)
	result, err := call.Execute(ctx, r, recorder, time, from, txID)
	require.NoError(err)
	require.Nil(result.(*EvmCallResult).Err)
	call.Keys = recorder.GetStateKeys()

	stateKeys := call.StateKeys(from, txID)
	storage := make(map[string][]byte, len(stateKeys))
	for key := range stateKeys {
		val, err := view.GetValue(ctx, []byte(key))
		if errors.Is(err, database.ErrNotFound) {
			continue
		}
		require.NoError(err)
		storage[key] = val
	}
	ts := tstate.New(0) // estimate of changed keys does not need to be accurate
	tsv := ts.NewView(stateKeys, storage)
	resultExecute, err := call.Execute(ctx, r, tsv, time, from, txID)
	require.NoError(err)
	require.Equal(result.(*EvmCallResult).UsedGas, resultExecute.(*EvmCallResult).UsedGas)
	require.Equal(result.(*EvmCallResult).Return, resultExecute.(*EvmCallResult).Return)

	tsv.Commit()
	view, err = ts.ExportMerkleDBView(ctx, tracer, view)
	require.NoError(err)
	return view
}

func InitializeAccount(ctx context.Context, mu state.Mutable, address codec.Address, balance uint64) error {

	newAccount := types.StateAccount{
		Nonce:    0,
		Balance:  uint256.NewInt(balance),
		Root:     common.Hash{},
		CodeHash: []byte{},
	}

	encoded, err := storage.EncodeAccount(&newAccount)
	if err != nil {
		return fmt.Errorf("failed to encode account: %w", err)
	}
	sender := ToEVMAddress(address)
	err = storage.SetAccount(ctx, mu, sender[:], encoded)
	if err != nil {
		return fmt.Errorf("failed to set account: %w", err)
	}
	return nil
}
