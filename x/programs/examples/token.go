// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func NewToken(log logging.Logger, engine *engine.Engine, programBytes []byte, db state.Mutable, cfg *runtime.Config, imports host.SupportedImports, maxUnits uint64) *Token {
	return &Token{
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
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	imports      host.SupportedImports
	db           state.Mutable
	maxUnits     uint64
	engine       *engine.Engine
}

func (t *Token) Run(ctx context.Context) error {
	rt := runtime.New(t.log, t.engine, t.imports, t.cfg)
	err := rt.Initialize(ctx, t.programBytes, t.maxUnits)
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
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	mem, err := rt.Memory()
	if err != nil {
		return err
	}

	programIDPtr, err := argumentToSmartPtr(programID, mem)
	if err != nil {
		return err
	}

	t.log.Debug("new token program created",
		zap.String("id", programID.String()),
	)

	// initialize program
	resp, err := rt.Call(ctx, "init", programIDPtr)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	t.log.Debug("init response",
		zap.Int64("init", resp[0]),
	)

	result, err := rt.Call(ctx, "get_total_supply", programIDPtr)
	if err != nil {
		return err
	}
	t.log.Debug("total supply",
		zap.Int64("minted", result[0]),
	)

	// generate alice keys
	_, aliceKey, err := newKey()
	if err != nil {
		return err
	}

	// write alice's key to stack and get pointer
	alicePtr, err := argumentToSmartPtr(aliceKey, mem)
	if err != nil {
		return err
	}

	// generate bob keys
	_, bobKey, err := newKey()
	if err != nil {
		return err
	}

	// write bob's key to stack and get pointer
	bobPtr, err := argumentToSmartPtr(bobKey, mem)
	if err != nil {
		return err
	}

	// check balance of bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// mint 100 tokens to alice
	mintAlice := int64(1000)
	mintAlicePtr, err := argumentToSmartPtr(mintAlice, mem)
	if err != nil {
		return err
	}

	_, err = rt.Call(ctx, "mint_to", programIDPtr, alicePtr, mintAlicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Int64("alice", mintAlice),
	)

	// check balance of alice
	result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", result[0]),
	)

	// check balance of bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", result[0]),
	)

	// transfer 50 from alice to bob
	transferToBob := int64(50)
	transferToBobPtr, err := argumentToSmartPtr(transferToBob, mem)
	if err != nil {
		return err
	}
	_, err = rt.Call(ctx, "transfer", programIDPtr, alicePtr, bobPtr, transferToBobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Int64("alice", transferToBob),
		zap.Int64("to bob", transferToBob),
	)

	onePtr, err := argumentToSmartPtr(int64(1), mem)
	if err != nil {
		return err
	}

	_, err = rt.Call(ctx, "transfer", programIDPtr, alicePtr, bobPtr, onePtr)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Int64("alice", 1),
		zap.Int64("to bob", 1),
	)

	// get balance alice
	result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", result[0]),
	)

	// get balance bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance", zap.Int64("bob", result[0]))

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
			To:     aliceKey,
			Amount: 10,
		},
		{
			To:     bobKey,
			Amount: 12,
		},
	}

	mintersPtr, err := argumentToSmartPtr(minters, mem)
	if err != nil {
		return err
	}

	// perform bulk mint
	_, err = rt.Call(ctx, "mint_to_many", programIDPtr, mintersPtr)
	if err != nil {
		return err
	}
	t.log.Debug("minted many",
		zap.Int32("alice", minters[0].Amount),
		zap.Int32("to bob", minters[1].Amount),
	)

	// get balance alice
	result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", result[0]),
	)

	// get balance bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance", zap.Int64("bob", result[0]))

	return nil
}

// RunShort performs the steps of initialization only, used for benchmarking.
func (t *Token) RunShort(ctx context.Context) error {
	rt := runtime.New(t.log, t.engine, t.imports, t.cfg)
	err := rt.Initialize(ctx, t.programBytes, t.maxUnits)
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
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	mem, err := rt.Memory()
	if err != nil {
		return err
	}

	programIDPtr, err := argumentToSmartPtr(programID, mem)
	if err != nil {
		return err
	}

	t.log.Debug("new token program created",
		zap.String("id", programID.String()),
	)

	// initialize program
	resp, err := rt.Call(ctx, "init", programIDPtr)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	t.log.Debug("init response",
		zap.Int64("init", resp[0]),
	)
	return nil
}
