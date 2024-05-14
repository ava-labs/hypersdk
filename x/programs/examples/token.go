// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

type TokenStateKey uint8

const (
	/// The total supply of the token. Key prefix 0x0.
	TotalSupply TokenStateKey = iota
	/// The name of the token. Key prefix 0x1.
	Name
	/// The symbol of the token. Key prefix 0x2.
	Symbol
	/// The balance of the token by address. Key prefix 0x3 + address.
	Balance
)

func NewToken(programID ids.ID, log logging.Logger, engine *engine.Engine, programBytes []byte, db state.Mutable, cfg *runtime.Config, imports host.SupportedImports, maxUnits uint64) *Token {
	return &Token{
		programID:    programID,
		log:          log,
		programBytes: programBytes,
		cfg:          cfg,
		imports:      imports,
		db:           db,
		maxUnits:     maxUnits,
		engine:       engine,
	}
}

type minter struct {
	// TODO: use a HyperSDK.Address instead
	To ed25519.PublicKey
	// note: a production program would use a uint64 for amount
	Amount int32
}

type Token struct {
	programID    ids.ID
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	imports      host.SupportedImports
	db           state.Mutable
	maxUnits     uint64
	engine       *engine.Engine
}

func (t *Token) Context() *program.Context {
	return &program.Context{
		ProgramID: t.programID,
	}
}

func (t *Token) ProgramID() ids.ID {
	return t.programID
}

func (t *Token) Run(ctx context.Context) error {
	rt := runtime.New(t.log, t.engine, t.imports, t.cfg)
	programContext := t.Context()

	err := rt.Initialize(ctx, programContext, t.programBytes, t.maxUnits)
	if err != nil {
		return err
	}

	balance, err := rt.Meter().GetBalance()
	if err != nil {
		return err
	}

	t.log.Debug("initial meter",
		zap.Uint64("balance", balance),
	)

	// simulate create program transaction
	programID := t.ProgramID()
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	mem, err := rt.Memory()
	if err != nil {
		return err
	}

	t.log.Debug("new token program created",
		zap.String("id", programID.String()),
	)

	// initialize program
	_, err = rt.Call(ctx, "init", programContext)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	_, err = rt.Call(ctx, "get_total_supply", programContext)
	if err != nil {
		return err
	}

	// generate alice keys
	alicePublicKey, err := newKey()
	if err != nil {
		return err
	}

	// write alice's key to stack and get pointer
	alicePtr, err := writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	// generate bob keys
	bobPublicKey, err := newKey()
	if err != nil {
		return err
	}

	// write bob's key to stack and get pointer
	bobPtr, err := writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	// check balance of bob
	_, err = rt.Call(ctx, "get_balance", programContext, bobPtr)
	if err != nil {
		return err
	}

	// mint 100 tokens to alice
	mintAlice := int64(1000)
	mintAlicePtr, err := writeToMem(mintAlice, mem)
	if err != nil {
		return err
	}

	_, err = rt.Call(ctx, "mint_to", programContext, alicePtr, mintAlicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Int64("alice", mintAlice),
	)

	alicePtr, err = writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	// check balance of alice
	_, err = rt.Call(ctx, "get_balance", programContext, alicePtr)
	if err != nil {
		return err
	}

	bobPtr, err = writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	// check balance of bob
	_, err = rt.Call(ctx, "get_balance", programContext, bobPtr)
	if err != nil {
		return err
	}

	// transfer 50 from alice to bob
	transferToBob := int64(50)
	transferToBobPtr, err := writeToMem(transferToBob, mem)
	if err != nil {
		return err
	}
	bobPtr, err = writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	alicePtr, err = writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	_, err = rt.Call(ctx, "transfer", programContext, alicePtr, bobPtr, transferToBobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Int64("alice", transferToBob),
		zap.Int64("to bob", transferToBob),
	)

	onePtr, err := writeToMem(int64(1), mem)
	if err != nil {
		return err
	}

	bobPtr, err = writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	alicePtr, err = writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	_, err = rt.Call(ctx, "transfer", programContext, alicePtr, bobPtr, onePtr)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Int64("alice", 1),
		zap.Int64("to bob", 1),
	)

	alicePtr, err = writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	// get balance alice
	_, err = rt.Call(ctx, "get_balance", programContext, alicePtr)
	if err != nil {
		return err
	}

	bobPtr, err = writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	// get balance bob
	_, err = rt.Call(ctx, "get_balance", programContext, bobPtr)
	if err != nil {
		return err
	}

	balance, err = rt.Meter().GetBalance()
	if err != nil {
		return err
	}

	t.log.Debug("remaining balance",
		zap.Uint64("unit", balance),
	)

	// combine alice and bobs addresses
	minters := []minter{
		{
			To:     alicePublicKey,
			Amount: 10,
		},
		{
			To:     bobPublicKey,
			Amount: 12,
		},
	}

	mintersPtr, err := writeToMem(minters, mem)
	if err != nil {
		return err
	}

	// perform bulk mint
	_, err = rt.Call(ctx, "mint_to_many", programContext, mintersPtr)
	if err != nil {
		return err
	}
	t.log.Debug("minted many",
		zap.Int32("alice", minters[0].Amount),
		zap.Int32("to bob", minters[1].Amount),
	)

	alicePtr, err = writeToMem(alicePublicKey, mem)
	if err != nil {
		return err
	}

	// get balance alice
	_, err = rt.Call(ctx, "get_balance", programContext, alicePtr)
	if err != nil {
		return err
	}

	bobPtr, err = writeToMem(bobPublicKey, mem)
	if err != nil {
		return err
	}

	// get balance bob
	_, err = rt.Call(ctx, "get_balance", programContext, bobPtr)
	if err != nil {
		return err
	}

	return nil
}

// RunShort performs the steps of initialization only, used for benchmarking.
func (t *Token) RunShort(ctx context.Context) error {
	rt := runtime.New(t.log, t.engine, t.imports, t.cfg)

	programContext := t.Context()

	err := rt.Initialize(ctx, programContext, t.programBytes, t.maxUnits)
	if err != nil {
		return err
	}

	balance, err := rt.Meter().GetBalance()
	if err != nil {
		return err
	}

	t.log.Debug("initial meter",
		zap.Uint64("balance", balance),
	)

	programID := t.ProgramID()
	// simulate create program transaction
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	t.log.Debug("new token program created",
		zap.String("id", programID.String()),
	)

	// initialize program
	_, err = rt.Call(ctx, "init", &program.Context{ProgramID: programID})
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	return nil
}

func (t *Token) GetUserBalanceFromState(ctx context.Context, programID ids.ID, userPublicKey ed25519.PublicKey) (res uint32, err error) {
	key := storage.ProgramPrefixKey(programID[:], append([]byte{uint8(Balance)}, userPublicKey[:]...))
	b, err := t.db.GetValue(ctx, key)
	if err != nil {
		return 0, err
	}
	err = borsh.Deserialize(&res, b)
	if err != nil {
		return 0, err
	}
	return res, nil
}
