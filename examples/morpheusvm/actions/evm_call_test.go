// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newLogger(prefix string) logging.Logger {
	return logging.NewLogger(
		prefix,
		logging.NewWrappedCore(
			logging.Info,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		),
	)
}

func TestContract(t *testing.T) {
	require := require.New(t)
	controller := gomock.NewController(t)
	data := common.Hex2Bytes("608060405234801561000f575f80fd5b506101438061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea264697066735822122000afd17ac37e0bb2b68b3ac973de3608be934fa6f2b2e31808f1502fc93a2f2d64736f6c63430008180033")

	sufficientGas := uint64(1000000)

	var from codec.Address
	copy(from[20:], []byte("112233"))
	tip := int64(0)
	txTip := big.NewInt(tip * params.GWei)
	baseFee := big.NewInt(0)
	feeCap := new(big.Int).Add(baseFee, txTip)
	call := &EvmCall{
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,

		logger: newLogger("test"),
	}

	rules := chain.NewMockRules(controller)
	rules.EXPECT().GetMaxBlockUnits().Return(
		genesis.Default().MaxBlockUnits,
	).AnyTimes()
	time := time.Now().UnixMilli()
	txID := ids.ID{}

	ctx := context.Background()
	tracer, err := trace.New(&trace.Config{Enabled: false})
	require.NoError(err)
	statedb, err := merkledb.New(ctx, memdb.New(), merkledb.Config{
		BranchFactor:                merkledb.BranchFactor16,
		RootGenConcurrency:          1,
		HistoryLength:               100,
		ValueNodeCacheSize:          units.MiB,
		IntermediateNodeCacheSize:   units.MiB,
		IntermediateWriteBufferSize: units.KiB,
		IntermediateWriteBatchSize:  units.KiB,
		Tracer:                      tracer,
	})
	require.NoError(err)
	mu := state.NewSimpleMutable(statedb)
	{
		success, actionCUs, output, warpMessage, err := call.Execute(
			ctx, rules, mu, time, from, txID, false,
		)
		require.Nil(err)
		require.Nil(warpMessage)
		require.True(success)
		fmt.Println("accessList:", call.toMessage(ToEVMAddress(from)).AccessList)

		fmt.Println(actionCUs)
		fmt.Println(common.Bytes2Hex(output))
	}

	contractAddress := crypto.CreateAddress(ToEVMAddress(from), 0)
	data = common.Hex2Bytes("6057361d000000000000000000000000000000000000000000000000000000000000002a")
	call = &EvmCall{
		To:        &contractAddress,
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,
		Nonce:     1,

		logger: newLogger("test"),
	}
	{
		success, actionCUs, output, warpMessage, err := call.Execute(
			ctx, rules, mu, time, from, txID, false,
		)
		require.Nil(err)
		require.Nil(warpMessage)
		require.True(success)
		fmt.Println("accessList:", call.toMessage(ToEVMAddress(from)).AccessList)

		fmt.Println(actionCUs)
		fmt.Println(common.Bytes2Hex(output))
	}

	data = common.Hex2Bytes("2e64cec1")
	call = &EvmCall{
		To:        &contractAddress,
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,
		Nonce:     2,

		logger: newLogger("test"),
	}
	require.NoError(mu.Commit(ctx))
	mu = state.NewSimpleMutable(statedb)
	{
		success, actionCUs, output, warpMessage, err := call.Execute(
			ctx, rules, mu, time, from, txID, false,
		)
		require.Nil(err)
		require.Nil(warpMessage)
		require.True(success)
		fmt.Println("accessList:", call.toMessage(ToEVMAddress(from)).AccessList)

		fmt.Println(actionCUs)
		fmt.Println(common.Bytes2Hex(output))
	}

	require.NoError(mu.Commit(ctx))
	newRoot, err := statedb.GetMerkleRoot(ctx)
	require.NoError(err)
	fmt.Println(newRoot)
}

func toExecutionResult(
	success bool,
	actionCUs uint64,
	output []byte,
	warpMessage *warp.UnsignedMessage,
	err error,
) *executionResult {
	return &executionResult{
		success:     success,
		actionCUs:   actionCUs,
		output:      output,
		warpMessage: warpMessage,
		err:         err,
	}
}

type executionResult struct {
	success     bool
	actionCUs   uint64
	output      []byte
	warpMessage *warp.UnsignedMessage
	err         error
}

func TestContractWithTracing(t *testing.T) {
	require := require.New(t)
	controller := gomock.NewController(t)
	data := common.Hex2Bytes("608060405234801561000f575f80fd5b506101438061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c80632e64cec1146100385780636057361d14610056575b5f80fd5b610040610072565b60405161004d919061009b565b60405180910390f35b610070600480360381019061006b91906100e2565b61007a565b005b5f8054905090565b805f8190555050565b5f819050919050565b61009581610083565b82525050565b5f6020820190506100ae5f83018461008c565b92915050565b5f80fd5b6100c181610083565b81146100cb575f80fd5b50565b5f813590506100dc816100b8565b92915050565b5f602082840312156100f7576100f66100b4565b5b5f610104848285016100ce565b9150509291505056fea264697066735822122000afd17ac37e0bb2b68b3ac973de3608be934fa6f2b2e31808f1502fc93a2f2d64736f6c63430008180033")

	sufficientGas := uint64(1000000)

	var from codec.Address
	copy(from[20:], []byte("112233"))
	tip := int64(0)
	txTip := big.NewInt(tip * params.GWei)
	baseFee := big.NewInt(0)
	feeCap := new(big.Int).Add(baseFee, txTip)
	call := &EvmCall{
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,

		logger: newLogger("test"),
	}

	r := chain.NewMockRules(controller)
	r.EXPECT().GetMaxBlockUnits().Return(
		genesis.Default().MaxBlockUnits,
	).AnyTimes()
	time := time.Now().UnixMilli()
	txID := ids.ID{}

	ctx := context.Background()
	tracer, err := trace.New(&trace.Config{Enabled: false})
	require.NoError(err)
	statedb, err := merkledb.New(ctx, memdb.New(), merkledb.Config{
		BranchFactor:                merkledb.BranchFactor16,
		RootGenConcurrency:          1,
		HistoryLength:               100,
		ValueNodeCacheSize:          units.MiB,
		IntermediateNodeCacheSize:   units.MiB,
		IntermediateWriteBufferSize: units.KiB,
		IntermediateWriteBatchSize:  units.KiB,
		Tracer:                      tracer,
	})
	require.NoError(err)

	traceAndExecute := func(call *EvmCall, view state.View) state.View {
		mu := state.NewSimpleMutable(view)
		traceAction := call.TraceAction()
		traceResult := toExecutionResult(
			traceAction.Execute(ctx, r, mu, time, from, txID, false),
		)
		require.NoError(traceResult.err)
		require.Nil(traceResult.warpMessage)
		require.True(traceResult.success)

		// now that the state keys are known we should be able to execute the action
		// in the environment with restricted state keys
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
		executionResult := toExecutionResult(
			call.Execute(ctx, r, tsv, time, from, txID, false),
		)
		require.NoError(executionResult.err)
		require.Nil(executionResult.warpMessage)
		require.True(executionResult.success)
		require.Equal(traceResult.actionCUs, executionResult.actionCUs)
		require.Equal(traceResult.output, executionResult.output)
		fmt.Println("accessList:", call.toMessage(ToEVMAddress(from)).AccessList)

		tsv.Commit()
		view, err = ts.ExportMerkleDBView(ctx, tracer, view)
		require.NoError(err)
		return view
	}

	view := state.View(statedb)
	view = traceAndExecute(call, view)

	contractAddress := crypto.CreateAddress(ToEVMAddress(from), 0)
	data = common.Hex2Bytes("6057361d000000000000000000000000000000000000000000000000000000000000002a")
	call = &EvmCall{
		To:        &contractAddress,
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,
		Nonce:     1,

		logger: newLogger("test"),
	}
	view = traceAndExecute(call, view)

	data = common.Hex2Bytes("2e64cec1")
	call = &EvmCall{
		To:        &contractAddress,
		Value:     common.Big0,
		GasLimit:  sufficientGas,
		Data:      data,
		GasFeeCap: feeCap,
		GasTipCap: txTip,
		GasPrice:  feeCap,
		Nonce:     2,

		logger: newLogger("test"),
	}
	_ = traceAndExecute(call, view)
}
