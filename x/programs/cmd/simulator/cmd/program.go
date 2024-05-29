// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
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

	id, err := programCreateFunc(ctx, db, *c.path)
	if err != nil {
		return newResponse(0), err
	}

	c.log.Debug("create program transaction successful", zap.String("id", id.String()))

	resp := newResponse(0)
	resp.setTimestamp(time.Now().Unix())
	return resp, nil
}

func (c *programCreateCmd) Happened() bool {
	return c.cmd.Happened()
}

// createProgram simulates a create program transaction and stores the program to disk.
func programCreateFunc(ctx context.Context, db *state.SimpleMutable, path string) (ids.ID, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return ids.Empty, err
	}

	// simulate create program transaction
	programID, err := generateRandomID()
	if err != nil {
		return ids.Empty, err
	}

	programCreateAction := actions.ProgramCreate{
		Program: programBytes,
	}

	// execute the action
	output, err := programCreateAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programID)
	if output != nil {
		response := multilineOutput(output)
		fmt.Println(response)
	}
	if err != nil {
		return ids.Empty, fmt.Errorf("program creation failed: %w", err)
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, err
	}

	return programID, nil
}

func programExecuteFunc(
	ctx context.Context,
	log logging.Logger,
	db *state.SimpleMutable,
	programID ids.ID,
	callParams []Parameter,
	function string,
	maxUnits uint64,
) (ids.ID, [][]byte, uint64, error) {
	// simulate create program transaction
	programTxID, err := generateRandomID()
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	bytes, err := SerializeParams(callParams)
	if err != nil {
		return ids.Empty, nil, 0, err
	}
	programExecuteAction := actions.ProgramExecute{
		ProgramID: programID,
		Function:  function,
		Params:    bytes,
		MaxUnits:  maxUnits,
		Log:       log,
	}

	// execute the action
	resp, err := programExecuteAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programTxID)
	if err != nil {
		response := multilineOutput(resp)
		if len(response) > 0 {
			err = fmt.Errorf("program execution failed: %s, err: %w", response, err)
		} else {
			err = fmt.Errorf("program execution failed, err: %w", err)
		}
		return ids.Empty, nil, 0, err
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	// get remaining balance from runtime meter
	balance, err := programExecuteAction.GetBalance()

	return programTxID, resp, balance, err
}

func SerializeParams(p []Parameter) ([]byte, error) {
	var bytes []byte
	for _, param := range p {
		switch v := param.Value.(type) {
		case []byte:
			bytes = append(bytes, v...)
		case ids.ID:
			bytes = append(bytes, v[:]...)
		case string:
			bytes = append(bytes, []byte(v)...)
		case uint64:
			bs := make([]byte, 8)
			binary.LittleEndian.PutUint64(bs, v)
			bytes = append(bytes, bs...)
		case uint32:
			bs := make([]byte, 4)
			binary.LittleEndian.PutUint32(bs, v)
			bytes = append(bytes, bs...)
		default:
			return nil, errors.New("unsupported data type")
		}
	}
	return bytes, nil
}

func multilineOutput(resp [][]byte) (response string) {
	for _, res := range resp {
		response += string(res) // + "\n"
	}
	return response
}
