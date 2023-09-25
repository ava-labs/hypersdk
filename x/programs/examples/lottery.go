// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"go.uber.org/zap"
)

func NewLottery(
	log logging.Logger,
	db state.Mutable,
	lotteryProgramBytes []byte,
	lotteryRuntimeCfg *runtime.Config,
	tokenProgramBytes []byte,
	tokenRuntimeCfg *runtime.Config,
	imports runtime.Imports) *Lottery {
	return &Lottery{
		db:                  db,
		imports:             imports,
		tokenProgramBytes:   tokenProgramBytes,
		tokenRuntimeCfg:     tokenRuntimeCfg,
		lotteryProgramBytes: lotteryProgramBytes,
		lotteryRuntimeCfg:   lotteryRuntimeCfg,
		log:                 log,
	}
}

type Lottery struct {
	db                  state.Mutable
	imports             runtime.Imports
	lotteryProgramBytes []byte
	lotteryRuntimeCfg   *runtime.Config
	tokenProgramBytes   []byte
	tokenRuntimeCfg     *runtime.Config
	log                 logging.Logger
}

func (t *Lottery) Run(ctx context.Context) error {
	tokenRuntime := runtime.New(t.log, t.tokenRuntimeCfg, t.imports)
	err := tokenRuntime.Initialize(ctx, t.tokenProgramBytes)
	if err != nil {
		return err
	}

	t.log.Debug("initial token meter",
		zap.Uint64("balance", tokenRuntime.Meter().GetBalance()),
	)

	// simulate create program transaction
	tokenProgramID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, t.db, tokenProgramID, t.tokenProgramBytes)
	if err != nil {
		return err
	}

	tokenProgramIDPtr, err := runtime.WriteBytes(tokenRuntime.Memory(), tokenProgramID[:])
	if err != nil {
		return err
	}
	t.log.Debug("new token program created",
		zap.String("id", tokenProgramID.String()),
	)

	// initialize token program
	_, err = tokenRuntime.Call(ctx, "init", tokenProgramIDPtr)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	// generate alice keys
	alicePtr, alice_pk, err := newKeyPtr(ctx, tokenRuntime)
	if err != nil {
		return err
	}

	// generate bob keys
	bobPtr, bob_pk, err := newKeyPtr(ctx, tokenRuntime)
	if err != nil {
		return err
	}

	// check balance of alice
	result, err := tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of bob
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// mint 100 tokens to alice
	mintAlice := uint64(1000)
	_, err = tokenRuntime.Call(ctx, "mint_to", tokenProgramIDPtr, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Uint64("alice", mintAlice),
	)

	// check balance of alice
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Uint64("alice", result[0]),
	)

	// check balance of bob
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// initialize lottery program
	lotteryRuntime := runtime.New(t.log, t.lotteryRuntimeCfg, t.imports)
	err = lotteryRuntime.Initialize(ctx, t.lotteryProgramBytes)
	if err != nil {
		return err
	}

	t.log.Debug("initial lottery meter",
		zap.Uint64("balance", lotteryRuntime.Meter().GetBalance()),
	)

	lotteryProgramID := ids.GenerateTestID()
	lotteryProgramIDPtr, err := runtime.WriteBytes(lotteryRuntime.Memory(), lotteryProgramID[:])
	if err != nil {
		return err
	}
	err = storage.SetProgram(ctx, t.db, lotteryProgramID, t.lotteryProgramBytes)
	if err != nil {
		return err
	}

	t.log.Debug("new lottery program created",
		zap.String("id", lotteryProgramID.String()),
	)

	// write all data to lottery runtime memory
	aliceLottoPtr, err := runtime.WriteBytes(lotteryRuntime.Memory(), alice_pk[:])
	if err != nil {
		return err
	}

	bobLottoPtr, err := runtime.WriteBytes(lotteryRuntime.Memory(), bob_pk[:])
	if err != nil {
		return err
	}

	tokenProgramIDPtr, err = runtime.WriteBytes(lotteryRuntime.Memory(), tokenProgramID[:])
	if err != nil {
		return err
	}

	t.log.Debug("alice key", zap.Any("result", alice_pk))

	// initialize lottery program
	result, err = lotteryRuntime.Call(ctx, "init", lotteryProgramIDPtr, tokenProgramIDPtr, aliceLottoPtr)
	if err != nil {
		return err
	}
	t.log.Debug("set", zap.Uint64("result", result[0]))

	maxUnits := uint64(100000)

	// play the lottery
	result, err = lotteryRuntime.Call(ctx, "play", lotteryProgramIDPtr, bobLottoPtr, maxUnits)
	if err != nil {
		return err
	}

	t.log.Debug("play result",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of alice
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of bob
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramIDPtr, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)
	return nil
}
