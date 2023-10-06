// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func NewToken(log logging.Logger, programBytes []byte, db state.Mutable, cfg *runtime.Config, imports runtime.SupportedImports) *Token {
	return &Token{
		log:          log,
		programBytes: programBytes,
		cfg:          cfg,
		imports:      imports,
		db:           db,
	}
}

type Token struct {
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	imports      runtime.SupportedImports
	db           state.Mutable
}

func (t *Token) Run(ctx context.Context) error {
	rt := runtime.New(t.log, t.cfg, t.imports)
	err := rt.Initialize(ctx, t.programBytes)
	if err != nil {
		return err
	}

	t.log.Debug("initial meter",
		zap.Uint64("balance", rt.Meter().GetBalance()),
	)

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	programIDPtr, err := runtime.WriteBytes(rt.Memory(), programID[:])
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
		zap.Uint64("init", resp[0]),
	)

	result, err := rt.Call(ctx, "get_total_supply", programIDPtr)
	if err != nil {
		return err
	}
	t.log.Debug("total supply",
		zap.Uint64("minted", result[0]),
	)

	// generate alice keys
	_, aliceKey, err := newKey()
	if err != nil {
		return err
	}

	// write alice's key to stack and get pointer
	alicePtr, err := newKeyPtr(ctx, aliceKey, rt)
	if err != nil {
		return err
	}

	// generate bob keys
	_, bobKey, err := newKey()
	if err != nil {
		return err
	}

	// write bob's key to stack and get pointer
	bobPtr, err := newKeyPtr(ctx, bobKey, rt)
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
	mintAlice := uint64(1000)
	_, err = rt.Call(ctx, "mint_to", programIDPtr, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Uint64("alice", mintAlice),
	)

	// check balance of alice
	result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Uint64("alice", result[0]),
	)

	// check balance of bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Uint64("bob", result[0]),
	)

	// transfer 50 from alice to bob
	transferToBob := uint64(50)
	_, err = rt.Call(ctx, "transfer", programIDPtr, alicePtr, bobPtr, transferToBob)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Uint64("alice", transferToBob),
		zap.Uint64("to bob", transferToBob),
	)

	_, err = rt.Call(ctx, "transfer", programIDPtr, alicePtr, bobPtr, 1)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Uint64("alice", transferToBob),
		zap.Uint64("to bob", transferToBob),
	)

	// get balance alice
	result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Uint64("alice", result[0]),
	)

	// get balance bob
	result, err = rt.Call(ctx, "get_balance", programIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance", zap.Uint64("bob", result[0]))

	t.log.Debug("remaining balance",
		zap.Uint64("unit", rt.Meter().GetBalance()),
	)

	return nil
}

// RunShort performs the steps of initialization only, used for benchmarking.
func (t *Token) RunShort(ctx context.Context) error {
	rt := runtime.New(t.log, t.cfg, t.imports)
	err := rt.Initialize(ctx, t.programBytes)
	if err != nil {
		return err
	}

	t.log.Debug("initial meter",
		zap.Uint64("balance", rt.Meter().GetBalance()),
	)

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, t.db, programID, t.programBytes)
	if err != nil {
		return err
	}

	programIDPtr, err := runtime.WriteBytes(rt.Memory(), programID[:])
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
		zap.Uint64("init", resp[0]),
	)
	return nil
}
