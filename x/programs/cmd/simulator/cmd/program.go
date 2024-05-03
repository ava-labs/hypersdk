// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/akamensky/argparse"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
)

var _ Cmd = &programCreateCmd{}

type programCreateCmd struct {
	cmd *argparse.Command

	db      *state.SimpleMutable
	keyName *string
	path    *string
}

func (c programCreateCmd) New(parser *argparse.Parser, db *state.SimpleMutable) Cmd {
	pcmd := parser.NewCommand("program-create", "Create a HyperSDK program transaction")

	c.keyName = pcmd.String("k", "key", &argparse.Options{
		Help:     "name of the key to use to deploy the program",
		Required: true,
	})
	// TODO use a file arg
	// c.path = pcmd.File()
	c.path = pcmd.String("p", "path", &argparse.Options{
		Help:     "path",
		Required: true,
	})

	return programCreateCmd{
		cmd: pcmd,
		db:  db,
	}
}

func (c programCreateCmd) Run(ctx context.Context, log logging.Logger, args []string) (err error) {
	exists, err := hasKey(ctx, c.db, *c.keyName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrNamedKeyNotFound, c.keyName)
	}

	id, err := programCreateFunc(ctx, c.db, *c.path)
	if err != nil {
		return err
	}

	utils.Outf("{{green}}create program transaction successful: {{/}}%s\n", id.String())

	return nil
}

func (c programCreateCmd) Happened() bool {
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
	success, _, output, err := programCreateAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programID)
	if output != nil {
		fmt.Println(string(output))
	}
	if !success {
		return ids.Empty, fmt.Errorf("program creation failed: %s", err)
	}
	if err != nil {
		return ids.Empty, err
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
	callParams []actions.CallParam,
	function string,
	maxUnits uint64,
) (ids.ID, []int64, uint64, error) {
	// simulate create program transaction
	programTxID, err := generateRandomID()

	if err != nil {
		return ids.Empty, nil, 0, err
	}

	programExecuteAction := actions.ProgramExecute{
		Function: function,
		Params:   callParams,
		MaxUnits: maxUnits,
		Log:      log,
	}

	// execute the action
	success, _, resp, err := programExecuteAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programTxID)

	if !success {
		return ids.Empty, nil, 0, fmt.Errorf("program execution failed: %s", string(resp))
	}
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	p := codec.NewReader(resp, len(resp))
	var result []int64
	for !p.Empty() {
		v := p.UnpackInt64(true)
		result = append(result, v)
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	// get remaining balance from runtime meter
	balance, err := programExecuteAction.GetBalance()

	return programTxID, result, balance, err
}
