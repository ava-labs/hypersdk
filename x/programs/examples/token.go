// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/hashmap"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"go.uber.org/zap"
)

func newKeyPtr(ctx context.Context, runtime runtime.Runtime) (int64, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return 0, ed25519.EmptyPublicKey, err
	}

	pk := priv.PublicKey()
	ptr, err := runtime.Memory().Alloc(ed25519.PublicKeyLen)
	if err != nil {
		return 0, ed25519.EmptyPublicKey, err
	}

	// write programID to memory which we will later pass to the program.
	err = runtime.Memory().Write(ptr, pk[:])
	if err != nil {
		return 0, ed25519.EmptyPublicKey, err
	}

	return int64(ptr), pk, err
}

func NewToken(log logging.Logger, programBytes []byte, cfg *runtime.Config, imports runtime.Imports) *TokenWasmtime {
	return &TokenWasmtime{
		log:          log,
		programBytes: programBytes,
		cfg:          cfg,
		imports:      imports,
	}
}

type TokenWasmtime struct {
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	imports      runtime.Imports
}

func (t *TokenWasmtime) Run(ctx context.Context) error {
	rt := runtime.New(t.log, t.cfg, t.imports)
	err := rt.Initialize(ctx, t.programBytes)
	if err != nil {
		return err
	}

	t.log.Debug("unit",
		zap.Uint64("balance", rt.Meter().GetBalance()),
	)

	// simulate create program transaction
	txID := ids.GenerateTestID()
	programID := int64(binary.BigEndian.Uint64(txID[:]))
	hashmap.AddProgramID(programID)

	// initialize program
	_, err = rt.Call(ctx, "init", programID)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	result, err := rt.Call(ctx, "get_total_supply", programID)
	if err != nil {
		return err
	}
	t.log.Debug("total supply",
		zap.Uint64("minted", result[0]),
	)

	// generate alice keys
	alicePtr, _, err := newKeyPtr(ctx, rt)
	if err != nil {
		return err
	}

	// generate bob keys
	bobPtr, _, err := newKeyPtr(ctx, rt)
	if err != nil {
		return err
	}

	// check balance of alice
	result, err = rt.Call(ctx, "get_balance", programID, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// mint 100 tokens to alice
	mintAlice := int64(100)
	_, err = rt.Call(ctx, "mint_to", programID, alicePtr, mintAlice)
	if err != nil {
		return err
	}
	t.log.Debug("minted",
		zap.Int64("alice", mintAlice),
	)

	// check balance of alice
	result, err = rt.Call(ctx, "get_balance", programID, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// check balance of bob
	result, err = rt.Call(ctx, "get_balance", programID, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("bob", int64(result[0])),
	)

	// transfer 50 from alice to bob
	transferToBob := int64(50)
	_, err = rt.Call(ctx, "transfer", programID, alicePtr, bobPtr, transferToBob)
	if err != nil {
		return err
	}
	t.log.Debug("transferred",
		zap.Int64("alice", transferToBob),
		zap.Int64("to bob", transferToBob),
	)

	// get balance alice
	result, err = rt.Call(ctx, "get_balance", programID, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance",
		zap.Int64("alice", int64(result[0])),
	)

	// get balance bob
	result, err = rt.Call(ctx, "get_balance", programID, bobPtr)
	if err != nil {
		return err
	}
	t.log.Debug("balance", zap.Int64("bob", int64(result[0])))

	t.log.Debug("remaining balance",
		zap.Uint64("unit", rt.Meter().GetBalance()),
	)

	return nil
}
