// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	// "errors"
	"fmt"
	"os"
	"time"

	"github.com/akamensky/argparse"
	// "github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ Cmd = (*programCreateCmd)(nil)

type programCreateCmd struct {
	cmd *argparse.Command

	log     logging.Logger
	keyName *string
	path    *string
}

func (c *programCreateCmd) New(parser *argparse.Parser) {
	c.cmd = parser.NewCommand("program-create", "Create a HyperSDK program transaction")
	c.keyName = c.cmd.String("k", "key", &argparse.Options{
		Help:     "name of the key to use to deploy the program",
		Required: true,
	})
	c.path = c.cmd.String("p", "path", &argparse.Options{
		Help:     "path",
		Required: true,
	})
}

func (c *programCreateCmd) Run(ctx context.Context, log logging.Logger, db *state.SimpleMutable, _ []string) (*Response, error) {
	c.log = log
	exists, err := hasKey(ctx, db, *c.keyName)
	if err != nil {
		return newResponse(0), err
	}
	if !exists {
		return newResponse(0), fmt.Errorf("%w: %s", ErrNamedKeyNotFound, *c.keyName)
	}

	programAddress, err := programCreateFunc(ctx, db, *c.path)
	if err != nil {
		return newResponse(0), err
	}

	c.log.Debug("create program transaction successful", zap.String("id", codec.ToHex(programAddress[:])))

	resp := newResponse(0)
	resp.setTimestamp(time.Now().Unix())
	return resp, nil
}

func (c *programCreateCmd) Happened() bool {
	return c.cmd.Happened()
}

// createProgram simulates a create program transaction and stores the program to disk.
func programCreateFunc(ctx context.Context, db *state.SimpleMutable, path string) (codec.Address, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return codec.EmptyAddress, err
	}

	// simulate create program transaction
	programID, err := generateRandomID()
	if err != nil {
		return codec.EmptyAddress, err
	}

	err = setProgram(ctx, db, programID, programBytes)
	if err != nil {
		response := multilineOutput([][]byte{utils.ErrBytes(err)})
		fmt.Println(response)
		return codec.EmptyAddress, fmt.Errorf("program creation failed: %w", err)
	}

	account, err := deployProgram(ctx, db, programID, []byte{})
	if err != nil {
		response := multilineOutput([][]byte{utils.ErrBytes(err)})
		fmt.Println(response)
		return codec.EmptyAddress, fmt.Errorf("program creation failed: %w", err)
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return codec.EmptyAddress, err
	}

	return account, nil
}

func programExecuteFunc(
	ctx context.Context,
	log logging.Logger,
	db *state.SimpleMutable,
	program codec.Address,
	callParams []Parameter,
	function string,
	maxUnits uint64,
) ([]byte, uint64, error) {
	// execute the action
	var bytes []byte
	for _, param := range callParams {
		bytes = append(bytes, param.Value...)
	}

	rt := runtime.NewRuntime(runtime.NewConfig(), log)
	callInfo := &runtime.CallInfo{
		State:        &programStateManager{Mutable: db},
		Actor:        codec.EmptyAddress,
		Program:      program,
		Fuel:         maxUnits,
		FunctionName: function,
		Params:       bytes,
	}
	result, err := rt.CallProgram(ctx, callInfo)
	remainingFuel := callInfo.RemainingFuel()
	if err != nil {
		response := string(result)
		return nil, 0, fmt.Errorf("program execution failed: %s, err: %w", response, err)
	}

	return result, remainingFuel, err
}

func multilineOutput(resp [][]byte) (response string) {
	for _, res := range resp {
		response += string(res) + "\n"
	}
	return response
}
