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
	Context       context.Context
	From          codec.Address
	Recipient     codec.Address
	SufficientGas uint64
	Rules         *genesis.Rules
	Timestamp     int64
	ActionID      ids.ID
	Tracer        trace.Tracer
	State         *SimpleMutable
	Nonce         uint64
	FactoryNonce  uint64
	ABIs          map[string]*ParsedABI
}

func NewTestContext() (*TestContext, error) {
	ctx := context.Background()

	from := codec.CreateAddress(uint8(1), ids.GenerateTestID())

	tracer, err := trace.New(trace.Config{Enabled: false})
	if err != nil {
		return nil, err
	}
	statedb, err := merkledb.New(ctx, memdb.New(), getTestMerkleConfig(tracer))
	if err != nil {
		return nil, err
	}

	mu := NewSimpleMutable(statedb)
	err = InitAccount(ctx, mu, from, uint64(10000000))
	if err != nil {
		return nil, err
	}

	abis, err := ParseABIs("../abis")
	if err != nil {
		return nil, err
	}

	recipient := codec.CreateAddress(uint8(2), ids.GenerateTestID())
	err = InitAccount(ctx, mu, recipient, 0)
	if err != nil {
		return nil, err
	}

	return &TestContext{
		Context:       ctx,
		SufficientGas: uint64(1000000),
		From:          from,
		Rules:         genesis.NewDefaultRules(),
		Timestamp:     time.Now().UnixMilli(),
		ActionID:      ids.GenerateTestID(),
		Tracer:        tracer,
		State:         mu,
		Nonce:         0,
		FactoryNonce:  0,
		ABIs:          abis,
		Recipient:     recipient,
	}, nil
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

type ParsedABI struct {
	ABI              abi.ABI
	Bytecode         []byte
	DeployedBytecode []byte
}

func ParseABIs(abiDir string) (map[string]*ParsedABI, error) {
	// Iterate over all files in directory
	files, err := os.ReadDir(abiDir)
	if err != nil {
		return nil, err
	}
	parsedABIs := make(map[string]*ParsedABI, len(files))
	for _, file := range files {
		// Parse file name
		parsedFileName := strings.SplitN(file.Name(), ".", 2)
		if len(parsedFileName) != 2 {
			return nil, fmt.Errorf("invalid file name: %s", file.Name())
		}
		// Parse each file
		parsedABI, err := ParseABI(abiDir, file)
		if err != nil {
			return nil, err
		}
		parsedABIs[parsedFileName[0]] = parsedABI
	}
	return parsedABIs, nil
}

func ParseABI(dir string, file os.DirEntry) (*ParsedABI, error) {
	// TODO: could we make creating the filepath cleaner?
	f, err := os.Open(dir + "/" + file.Name())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	mapData := make(map[string]json.RawMessage)
	if err := json.NewDecoder(f).Decode(&mapData); err != nil {
		return nil, err
	}

	bytecodeHexData, err := parseBytecode("bytecode", mapData["bytecode"])
	if err != nil {
		return nil, err
	}

	deployedBytecodeHexData, err := parseBytecode("deployedBytecode", mapData["deployedBytecode"])
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(string(mapData["abi"])))
	if err != nil {
		return nil, err
	}

	return &ParsedABI{
		ABI:              abi,
		Bytecode:         bytecodeHexData,
		DeployedBytecode: deployedBytecodeHexData,
	}, nil
}

func parseBytecode(namespace string, msg json.RawMessage) ([]byte, error) {
	var bytecodeHex string
	if err := json.Unmarshal(msg, &bytecodeHex); err != nil {
		// attempt to unmarshal map
		var mapData map[string]json.RawMessage
		if err := json.Unmarshal(msg, &mapData); err != nil {
			return nil, err
		}
		var str string
		object, ok := mapData["object"]
		if !ok {
			return nil, fmt.Errorf("missing object field in %s", namespace)
		}
		if err := json.Unmarshal(object, &str); err != nil {
			return nil, err
		}
		bytecodeHex = str
	}
	bytecodeHex = strings.TrimLeft(bytecodeHex, "0x")
	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, err
	}
	return bytecode, nil
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
