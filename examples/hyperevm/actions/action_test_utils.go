// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/coreth/accounts/abi"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state"
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
	State           *SimpleMutable
	Nonce           uint64
	FactoryNonce    uint64
	TestContractABI *ParsedABI
	FactoryABI      *ParsedABI
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

	mu := NewSimpleMutable(statedb)
	err = InitAccount(ctx, mu, from, uint64(10000000))
	if err != nil {
		panic(err)
	}

	testContractABI, err := NewABI("../abis/TestContract.json")
	if err != nil {
		panic(err)
	}
	testContractFactoryABI, err := NewABI("../abis/ContractFactory.json")
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
	sender := storage.ToEVMAddress(address)
	err = storage.SetAccount(ctx, mu, sender, encoded)
	if err != nil {
		return fmt.Errorf("failed to set account: %w", err)
	}
	return nil
}

func GetStateKeys(ctx context.Context, call *EvmCall) state.Keys {
	return call.Keys
}

type ParsedABI struct {
	ABI              abi.ABI
	Bytecode         []byte
	DeployedBytecode []byte
}

type rawJSON string

func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = rawJSON(data)
	return nil
}

func (r rawJSON) AsString() string {
	return string(r[1 : len(r)-1])
}

func NewABI(compiledFn string) (*ParsedABI, error) {
	f, err := os.Open(compiledFn)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]rawJSON)
	if err := json.NewDecoder(f).Decode(&mapData); err != nil {
		return nil, err
	}

	bytecodeHex := mapData["bytecode"].AsString()
	bytecodeHex = strings.TrimLeft(bytecodeHex, "0x")
	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, err
	}

	var deployedBytecode []byte
	if _, ok := mapData["deployedBytecode"]; ok {
		deployedBytecodeHex := mapData["deployedBytecode"].AsString()
		deployedBytecodeHex = strings.TrimLeft(deployedBytecodeHex, "0x")
		deployedBytecode, err = hex.DecodeString(deployedBytecodeHex)
		if err != nil {
			return nil, err
		}
	} else {
		deployedBytecode = bytecode
	}

	abi, err := abi.JSON(strings.NewReader(string(mapData["abi"])))
	if err != nil {
		return nil, err
	}
	return &ParsedABI{ABI: abi, Bytecode: bytecode, DeployedBytecode: deployedBytecode}, nil
}

func (p *ParsedABI) BytecodeHex() string {
	return hex.EncodeToString(p.Bytecode)
}

func (p *ParsedABI) DeployedBytecodeHex() string {
	return hex.EncodeToString(p.DeployedBytecode)
}

var _ state.Mutable = (*SimpleMutable)(nil)

type SimpleMutable struct {
	v state.View

	changes map[string]maybe.Maybe[[]byte]
}

func NewSimpleMutable(v state.View) *SimpleMutable {
	return &SimpleMutable{v, make(map[string]maybe.Maybe[[]byte])}
}

func (s *SimpleMutable) GetValue(ctx context.Context, k []byte) ([]byte, error) {
	if v, ok := s.changes[string(k)]; ok {
		if v.IsNothing() {
			return nil, database.ErrNotFound
		}
		return v.Value(), nil
	}
	return s.v.GetValue(ctx, k)
}

func (s *SimpleMutable) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = maybe.Some(v)
	return nil
}

func (s *SimpleMutable) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = maybe.Nothing[[]byte]()
	return nil
}

func (s *SimpleMutable) Commit(ctx context.Context) error {
	view, err := s.v.NewView(ctx, merkledb.ViewChanges{MapOps: s.changes})
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}
