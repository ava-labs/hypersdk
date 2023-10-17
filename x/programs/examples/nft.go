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

func NewNFT(log logging.Logger, programBytes []byte, cfg *runtime.Config, imports runtime.SupportedImports, db state.Mutable) *NFT {
	return &NFT{
		log:          log,
		programBytes: programBytes,
		cfg:          cfg,
		imports:      imports,
		db:           db,
	}
}

type NFT struct {
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	imports      runtime.SupportedImports
	db           state.Mutable
}

func (t *NFT) Run(ctx context.Context) error {
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

	// initialize program
	resp, err := rt.Call(ctx, "init", programIDPtr)
	if err != nil {
		return fmt.Errorf("failed to initialize program: %w", err)
	}

	t.log.Debug("init response",
		zap.Uint64("init", resp[0]),
	)

	_, aliceKey, err := newKey()
	if err != nil {
		return err
	}

	// write alice's key to stack and get pointer
	alicePtr, err := newKeyPtr(ctx, aliceKey, rt)
	if err != nil {
		return err
	}

	// mint 1 token, send to alice
	_, err = rt.Call(ctx, "mint", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	t.log.Debug("minted")

	return nil
}
