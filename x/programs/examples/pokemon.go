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

func NewPokemon(log logging.Logger, programBytes []byte, maxFee uint64, costMap map[string]uint64) *Pokemon {
	return &Pokemon{
		log:          log,
		programBytes: programBytes,
		maxFee:       maxFee,
		costMap:      costMap,
	}
}

type Pokemon struct {
	log          logging.Logger
	programBytes []byte

	// metering
	maxFee  uint64
	costMap map[string]uint64
}

func (t *Pokemon) Run(ctx context.Context) error {
	meter := runtime.NewMeter(t.log, t.maxFee, t.costMap)
	db := utils.NewTestDB()
	store := newProgramStorage(db)

	runtime := runtime.New(t.log, meter, store)
	contractId, err := runtime.Create(ctx, t.programBytes)
	if err != nil {
		return err
	}
	t.log.Debug("initial cost",
		zap.Int("gas", 0),
	)

	// generate alice keys
	alicePtr, _, err := newKeyPtr(ctx, runtime)
	if err != nil {
		return err
	}
	// generate bob keys
	bobPtr, _, err := newKeyPtr(ctx, runtime)
	if err != nil {
		return err
	}

	// deallocate bytes
	defer func() {
		_, err = runtime.Call(ctx, "dealloc", alicePtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate alice ptr",
				zap.Error(err),
			)
		}
		_, err = runtime.Call(ctx, "dealloc", bobPtr, ed25519.PublicKeyLen)
		if err != nil {
			t.log.Error("failed to deallocate bob ptr",
				zap.Error(err),
			)
		}
	}()

	// check balance of bob
	result, err := runtime.Call(ctx, "catch", contractId, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("Catch Result",
		zap.Int64("Success", int64(result[0])),
	)

	// get owned
	result, err = runtime.Call(ctx, "get_owned", contractId, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("Result",
		zap.Int64("Success", int64(result[0])),
	)

	return nil
}
