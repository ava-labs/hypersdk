// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/x/programs/examples/imports/hashmap"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"go.uber.org/zap"
)

func NewLottery(log logging.Logger, programBytes []byte, lotteryCfg, tokenCfg *runtime.Config, imports runtime.Imports) *Lottery {
	return &Lottery{
		log:          log,
		programBytes: programBytes,
		tokenCfg:     tokenCfg,
		lotteryCfg:   lotteryCfg,
		imports:      imports,
	}
}

type Lottery struct {
	log          logging.Logger
	programBytes []byte
	lotteryCfg   *runtime.Config
	tokenCfg     *runtime.Config
	imports      runtime.Imports
}

func (t *Lottery) Run(ctx context.Context) error {
	tokenRuntime := runtime.New(t.log, t.tokenCfg, t.imports)
	err := tokenRuntime.Initialize(ctx, t.programBytes)
	if err != nil {
		return err
	}

	t.log.Debug("unit",
		zap.Uint64("balance", tokenRuntime.Meter().GetBalance()),
	)

	// simulate create program transaction
	newTokenProgramID := ids.GenerateTestID()
	tokenProgramId := int64(binary.BigEndian.Uint64(newTokenProgramID[:]))
	hashmap.AddProgramID(tokenProgramId)

	t.log.Debug("initial cost",
		zap.Int("gas", 0),
	)

	// initialize program
	_, err = tokenRuntime.Call(ctx, "init", tokenProgramId)
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
	result, err := tokenRuntime.Call(ctx, "get_balance", tokenProgramId, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of bob
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramId, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// mint 100 tokens to alice
	mintAlice := int64(1000)
	_, err = tokenRuntime.Call(ctx, "mint_to", tokenProgramId, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Int64("alice", mintAlice),
	)

	// check balance of alice
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramId, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// initialize lottery program
	lotteryRuntime := runtime.New(t.log, t.lotteryCfg, t.imports)
	newLotteryID := ids.GenerateTestID()
	lotteryProgramId := int64(binary.BigEndian.Uint64(newLotteryID[:]))
	hashmap.AddProgramID(lotteryProgramId)

	// initialize program
	_, err = tokenRuntime.Call(ctx, "init", lotteryProgramId)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	t.log.Debug("lottery program id", zap.Int64("id", lotteryProgramId))
	// set the program_id in store to the lottery bytes

	aliceLottoPtr, err := runtime.WriteBytes(lotteryRuntime.Memory(), alice_pk[:])
	if err != nil {
		return err
	}

	bobLottoPtr, err := runtime.WriteBytes(lotteryRuntime.Memory(), bob_pk[:])
	if err != nil {
		return err
	}

	// set the library program
	_, err = lotteryRuntime.Call(ctx, "set", lotteryProgramId, tokenProgramId, aliceLottoPtr)
	if err != nil {
		return err
	}

	// play the lottery
	result, err = lotteryRuntime.Call(ctx, "play", lotteryProgramId, bobLottoPtr)
	if err != nil {
		return err
	}
	t.log.Debug("set", zap.Uint64("result", result[0]))

	// check balance of alice
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramId, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of bob
	result, err = tokenRuntime.Call(ctx, "get_balance", tokenProgramId, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)
	return nil
}
