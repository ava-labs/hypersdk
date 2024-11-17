package actions

import (
	"context"
	"errors"
	"fmt"
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
	utils "github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type TestContext struct {
	Context         context.Context
	From            codec.Address
	Recipient       codec.Address
	SufficientGas   uint64
	Rules           *genesis.Rules
	Timestamp       int64
	ActionID        ids.ID
	Tracer          trace.Tracer
	State           *state.SimpleMutable
	Nonce           uint64
	FactoryNonce    uint64
	TestContractABI *utils.ParsedABI
	FactoryABI      *utils.ParsedABI
}

func NewTestContext() *TestContext {

	ctx := context.Background()

	from := codec.CreateAddress(uint8(1), ids.GenerateTestID())

	tracer, err := trace.New(trace.Config{Enabled: false})
	if err != nil {
		panic(err)
	}
	statedb, err := merkledb.New(ctx, memdb.New(), getTestMerkleConfig(tracer))
	if err != nil {
		panic(err)
	}

	mu := state.NewSimpleMutable(statedb)
	err = InitAccount(ctx, mu, from, uint64(10000000))
	if err != nil {
		panic(err)
	}

	// todo: remove hardhat and only keep the abi files after its done
	testContractABI, err := utils.NewABI("../contracts/artifacts/contracts/Test.sol/TestContract.json")
	if err != nil {
		panic(err)
	}
	testContractFactoryABI, err := utils.NewABI("../contracts/artifacts/contracts/Test.sol/ContractFactory.json")
	if err != nil {
		panic(err)
	}

	recipient := codec.CreateAddress(uint8(2), ids.GenerateTestID())
	err = InitAccount(ctx, mu, recipient, 0)
	if err != nil {
		panic(err)
	}

	return &TestContext{
		Context:         ctx,
		SufficientGas:   uint64(1000000),
		From:            from,
		Rules:           genesis.NewDefaultRules(),
		Timestamp:       time.Now().UnixMilli(),
		ActionID:        ids.GenerateTestID(),
		Tracer:          tracer,
		State:           mu,
		Nonce:           0,
		FactoryNonce:    0,
		TestContractABI: testContractABI,
		FactoryABI:      testContractFactoryABI,
		Recipient:       recipient,
	}
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

func InitAccount(ctx context.Context, mu state.Mutable, address codec.Address, balance uint64) error {

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
	sender := storage.ConvertAddress(address)
	err = storage.SetAccount(ctx, mu, sender, encoded)
	if err != nil {
		return fmt.Errorf("failed to set account: %w", err)
	}
	return nil
}

func GetStateKeys(ctx context.Context, call *EvmCall) state.Keys {
	return call.Keys
}

func TraceAndExecute(ctx context.Context, require *require.Assertions, call *EvmCall, view state.View, r chain.Rules, time int64, from codec.Address, txID ids.ID, tracer trace.Tracer) state.View {
	mu := state.NewSimpleMutable(view)
	err := InitAccount(ctx, mu, from, uint64(10000000))
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
