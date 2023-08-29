// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"

	"go.uber.org/zap"
)

func NewLottery(log logging.Logger, lotteryProgramBytes []byte, tokenProgramBytes []byte, maxFee uint64, costMap map[string]uint64) *Lottery {
	return &Lottery{
		log:                 log,
		lotteryProgramBytes: lotteryProgramBytes,
		tokenProgramBytes:   tokenProgramBytes,
		maxFee:              maxFee,
		costMap:             costMap,
	}
}

type Lottery struct {
	log                 logging.Logger
	lotteryProgramBytes []byte
	tokenProgramBytes   []byte

	// metering
	maxFee  uint64
	costMap map[string]uint64
}

func (t *Lottery) Run(ctx context.Context) error {

	meter := runtime.NewMeter(t.log, t.maxFee, t.costMap)
	db := utils.NewTestDB()
	store := newProgramStorage(db)

	tokenRuntime := runtime.New(t.log, meter, store)
	err := tokenRuntime.Initialize(ctx, t.tokenProgramBytes)
	if err != nil {
		return err
	}

	result, err := tokenRuntime.Call(ctx, "init_program")
	if err != nil {
		return err
	}
	t.log.Debug("initial cost",
		zap.Int("gas", 0),
	)

	tokenProgramId := result[0]

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

	// deallocate bytes
	defer func() {
		_, err = tokenRuntime.Call(ctx, "dealloc", alicePtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate alice ptr",
				zap.Error(err),
			)
		}
		_, err = tokenRuntime.Call(ctx, "dealloc", bobPtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate bob ptr",
				zap.Error(err),
			)
		}
	}()

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

	// mint 100 tokens to alice
	mintAlice := uint64(1000)
	_, err = tokenRuntime.Call(ctx, "mint_to", tokenProgramId, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Uint64("alice", mintAlice),
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
	lotteryRuntime := runtime.New(t.log, meter, store)
	err = lotteryRuntime.Initialize(ctx, t.lotteryProgramBytes)
	if err != nil {
		return err
	}

	result, err = lotteryRuntime.Call(ctx, "init_program")
	if err != nil {
		return err
	}
	lotteryProgramId := result[0]
	t.log.Debug("lottery program id", zap.Uint64("id", lotteryProgramId))
	runtime.GlobalStorage.Programs[uint32(tokenProgramId)] = t.tokenProgramBytes
	// set the program_id in store to the lottery bytes

	aliceLottoPtr, err := lotteryRuntime.WriteGuestBuffer(ctx, alice_pk[:])
	if err != nil {
		return err
	}

	bobLottoPtr, err := lotteryRuntime.WriteGuestBuffer(ctx, bob_pk[:])
	if err != nil {
		return err
	}

	// deallocate bytes
	defer func() {
		_, err = lotteryRuntime.Call(ctx, "dealloc", aliceLottoPtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate alice ptr",
				zap.Error(err),
			)
		}
		_, err = lotteryRuntime.Call(ctx, "dealloc", bobLottoPtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate bob ptr",
				zap.Error(err),
			)
		}
	}()

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
