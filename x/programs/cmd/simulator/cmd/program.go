// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	hutils "github.com/ava-labs/hypersdk/utils"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
	xconsts "github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/consts"
)

func newProgramCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program",
		Short: "Manage HyperSDK programs",
	}

	// add subcommands
	cmd.AddCommand(
		newProgramCreateCmd(log, db),
	)

	return cmd
}

type programCreate struct {
	db      *state.SimpleMutable
	keyName string
	path    string
	id      codec.LID
}

func newProgramCreateCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	p := &programCreate{
		db: db,
	}
	cmd := &cobra.Command{
		Use:   "create --path [path] --key [key name]",
		Short: "Create a HyperSDK program transaction",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := p.Init(args)
			if err != nil {
				return err
			}
			err = p.Verify(cmd.Context())
			if err != nil {
				return err
			}
			err = p.Run(cmd.Context())
			if err != nil {
				return err
			}

			hutils.Outf("{{green}}create program transaction successful: {{/}}%s\n", p.id.String())
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&p.keyName, "key", "k", p.keyName, "name of the key to use to deploy the program")
	cmd.MarkPersistentFlagRequired("key")

	return cmd
}

func (p *programCreate) Init(args []string) error {
	return nil
}

func (p *programCreate) Verify(ctx context.Context) error {
	exists, err := hasKey(ctx, p.db, p.keyName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrNamedKeyNotFound, p.keyName)
	}

	return err
}

func (p *programCreate) Run(ctx context.Context) (err error) {
	p.id, err = programCreateFunc(ctx, p.db, p.path)
	if err != nil {
		return err
	}

	return nil
}

// createProgram simulates a create program transaction and stores the program to disk.
func programCreateFunc(ctx context.Context, db *state.SimpleMutable, path string) (codec.LID, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return codec.EmptyAddress, err
	}

	// simulate create program transaction
	id, err := generateRandomID()
	if err != nil {
		return codec.EmptyAddress, err
	}
	programID := codec.CreateLID(0, id)

	programCreateAction := actions.ProgramCreate{
		Program: programBytes,
	}

	// execute the action
	success, _, outputs, err := programCreateAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programID)
	var resultOutputs string
	for i := 0; i < len(outputs); i++ {
		for j := 0; j < len(outputs[i]); j++ {
			resultOutputs += fmt.Sprintf(" %s", string(outputs[i][j]))
		}
	}
	if len(resultOutputs) > 0 {
		fmt.Println(resultOutputs)
	}
	if !success {
		return codec.EmptyAddress, fmt.Errorf("program creation failed: %s", err)
	}
	if err != nil {
		return codec.EmptyAddress, err
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return codec.EmptyAddress, err
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
) (codec.LID, []int64, uint64, error) {
	// simulate create program transaction
	programTxID, err := generateRandomID()
	if err != nil {
		return codec.EmptyAddress, nil, 0, err
	}
	programActionID := codec.CreateLID(0, programTxID)

	programExecuteAction := actions.ProgramExecute{
		Function: function,
		Params:   callParams,
		MaxUnits: maxUnits,
		Log:      log,
	}

	// execute the action
	success, _, resp, err := programExecuteAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programActionID)

	if !success {
		var respOutput string
		for i := 0; i < len(resp); i++ {
			respOutput += fmt.Sprintf(" %s", string(resp[i]))
		}
		return codec.EmptyAddress, nil, 0, fmt.Errorf("program execution failed: %s", respOutput)
	}
	if err != nil {
		return codec.EmptyAddress, nil, 0, err
	}

	// TODO: I don't think this is right
	size := 0
	for i := 1; i < len(resp); i++ {
		size += consts.IntLen + codec.BytesLen(resp[i])
	}
	p := codec.NewWriter(size, consts.MaxInt)
	var result []int64
	for !p.Empty() {
		v := p.UnpackInt64(true)
		result = append(result, v)
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return codec.EmptyAddress, nil, 0, err
	}

	// get remaining balance from runtime meter
	balance, err := programExecuteAction.GetBalance()

	return programActionID, result, balance, err
}
