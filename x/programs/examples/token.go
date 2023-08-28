// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"

	"go.uber.org/zap"
)

func NewToken(log logging.Logger, programBytes []byte, maxFee uint64, costMap map[string]uint64) *Token {
	return &Token{
		log:          log,
		programBytes: programBytes,
		maxFee:       maxFee,
		costMap:      costMap,
	}
}

type Token struct {
	log          logging.Logger
	programBytes []byte

	// metering
	maxFee  uint64
	costMap map[string]uint64
}

func (t *Token) Run(ctx context.Context) error {
	// functions exported in this example
	functions := []string{
		"get_total_supply",
		"mint_to",
		"get_balance",
		"transfer",
		"alloc",
		"dealloc",
		"init_program",
	}

	meter := runtime.NewMeter(t.log, t.maxFee, t.costMap)
	db := utils.NewTestDB()
	store := newProgramStorage(db)

	runtime := runtime.New(t.log, meter, store)
	err := runtime.Initialize(ctx, t.programBytes, functions)
	if err != nil {
		return err
	}

	result, err := runtime.Call(ctx, "init_program")
	if err != nil {
		return err
	}
	t.log.Debug("initial cost",
		zap.Int("gas", 0),
	)

	contract_id := result[0]
	result, err = runtime.Call(ctx, "get_total_supply", contract_id)
	if err != nil {
		return err
	}
	t.log.Debug("total supply",
		zap.Uint64("minted", result[0]),
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

	// check balance of alice
	result, err = runtime.Call(ctx, "get_balance", contract_id, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// mint 100 tokens to alice
	mintAlice := uint64(100)
	_, err = runtime.Call(ctx, "mint_to", contract_id, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Uint64("alice", mintAlice),
	)

	// check balance of alice
	result, err = runtime.Call(ctx, "get_balance", contract_id, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

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
	result, err = runtime.Call(ctx, "get_balance", contract_id, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// transfer 50 from alice to bob
	transferToBob := uint64(50)
	_, err = runtime.Call(ctx, "transfer", contract_id, alicePtr, bobPtr, transferToBob)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Uint64("alice", transferToBob),
		zap.Uint64("to bob", transferToBob),
	)

	// get balance alice
	result, err = runtime.Call(ctx, "get_balance", contract_id, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// get balance bob
	result, err = runtime.Call(ctx, "get_balance", contract_id, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance", zap.Int64("bob", int64(result[0])))

	return nil
}

func newKeyPtr(ctx context.Context, runtime runtime.Runtime) (uint64, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return 0, ed25519.EmptyPublicKey, err
	}

	pk := priv.PublicKey()
	ptr, err := runtime.WriteGuestBuffer(ctx, pk[:])
	return ptr, pk, err
}

// writeString writes a string to guest memory and returns the pointer to the string.
// The string is padded with 0s to fit 32 bytes.
func writeString(ctx context.Context, runtime runtime.Runtime, str string) (uint64, error) {
	if len(str) > 32 {
		return 0, fmt.Errorf("length of string %s exceeds 32 bytes", str)
	}
	bytes := [32]byte{}
	// push string to bytes
	copy(bytes[:], str)
	return runtime.WriteGuestBuffer(ctx, bytes[:])
}
