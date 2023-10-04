// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func NewCounter(
	log logging.Logger,
	programBytes []byte,
	db state.Mutable,
	cfg *runtime.Config,
	cfg2 *runtime.Config,
	imports runtime.SupportedImports,
) *Counter {
	return &Counter{
		log:          log,
		programBytes: programBytes,
		cfg:          cfg,
		cfg2:         cfg2,
		imports:      imports,
		db:           db,
	}
}

type Counter struct {
	log          logging.Logger
	programBytes []byte
	cfg          *runtime.Config
	cfg2         *runtime.Config
	imports      runtime.SupportedImports
	db           state.Mutable
}

func (c *Counter) Run(ctx context.Context) error {
	rt := runtime.New(c.log, c.cfg, c.imports)
	err := rt.Initialize(ctx, c.programBytes)
	if err != nil {
		return err
	}

	c.log.Debug("initial meter",
		zap.Uint64("balance", rt.Meter().GetBalance()),
	)

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, c.db, programID, c.programBytes)
	if err != nil {
		return err
	}

	programIDPtr, err := runtime.WriteBytes(rt.Memory(), programID[:])
	if err != nil {
		return err
	}

	c.log.Debug("new counter program created",
		zap.String("id", programID.String()),
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

	// create counter for alice on program 1
	_, err = rt.Call(ctx, "initialize_address", programIDPtr, alicePtr)
	if err != nil {
		return err
	}

	result, err := rt.Call(ctx, "get_value", programIDPtr, alicePtr)
	if err != nil {
		return err
	}
	c.log.Debug("count",
		zap.Uint64("alice", result[0]),
	)

	// initialize second runtime to create second counter program
	rt2 := runtime.New(c.log, c.cfg2, c.imports)
	err = rt2.Initialize(ctx, c.programBytes)
	if err != nil {
		return err
	}

	// simulate creating second program transaction
	program2ID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, c.db, program2ID, c.programBytes)
	if err != nil {
		return err
	}

	programID2Ptr, err := runtime.WriteBytes(rt2.Memory(), program2ID[:])
	if err != nil {
		return err
	}

	c.log.Debug("new counter program created",
		zap.String("id", program2ID.String()),
	)

	// write alice's key to stack and get pointer
	alicePtr2, err := newKeyPtr(ctx, aliceKey, rt2)
	if err != nil {
		return err
	}

	_, err = rt2.Call(ctx, "initialize_address", programID2Ptr, alicePtr2)
	if err != nil {
		return err
	}

	// increment alice's counter on program 2 by 10
	_, err = rt2.Call(ctx, "inc", programID2Ptr, alicePtr2, 10)
	if err != nil {
		return err
	}

	result, err = rt2.Call(ctx, "get_value", programID2Ptr, alicePtr2)
	if err != nil {
		return err
	}

	c.log.Debug("count program 2",
		zap.Uint64("alice", result[0]),
	)

	// stop 2nd runtime as work is done
	rt2.Stop()

	// increment alice's counter on program 1
	_, err = rt.Call(ctx, "inc", programIDPtr, alicePtr, 1)
	if err != nil {
		return err
	}

	result, err = rt.Call(ctx, "get_value", programIDPtr, alicePtr)
	if err != nil {
		return err
	}

	c.log.Debug("count program 1",
		zap.Uint64("alice", result[0]),
	)

	// write program id 2 to stack of program 1
	programID2Ptr, err = runtime.WriteBytes(rt.Memory(), program2ID[:])
	if err != nil {
		return err
	}

	// set the max units to the current balance
	maxUnits := uint64(20000)
	caller := programIDPtr
	target := programID2Ptr

	// increment alice's counter on program 2 by 5 from program 1
	_, err = rt.Call(ctx, "inc_external", caller, target, maxUnits, alicePtr, 5)
	if err != nil {
		return err
	}

	result, err = rt.Call(ctx, "get_value_external", caller, target, maxUnits, alicePtr)
	if err != nil {
		return err
	}

	c.log.Debug("count from program 2",
		zap.Uint64("alice", result[0]),
	)

	c.log.Debug("remaining balance",
		zap.Uint64("unit", rt.Meter().GetBalance()),
	)

	return nil
}
