// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	hutils "github.com/ava-labs/hypersdk/utils"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
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
	id      ids.ID
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
	_, outputs, err := programCreateAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programID)
	if err != nil {
		return ids.Empty, fmt.Errorf("program creation failed: %w", err)
	}
	if len(outputs) > 0 {
		var results []string
		for _, output := range outputs {
			results = append(results, string(output))
		}
		fmt.Println(results)
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
	programActionID, err := generateRandomID()
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
	_, resp, err := programExecuteAction.Execute(ctx, nil, db, 0, codec.EmptyAddress, programActionID)
	if err != nil {
		return ids.Empty, nil, 0, fmt.Errorf("program execution failed: %w", err)
	}

	var result []int64
	for _, r := range resp {
		p := codec.NewReader(r, len(r))
		for !p.Empty() {
			v := p.UnpackInt64(true)
			result = append(result, v)
		}
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, nil, 0, err
	}

	// get remaining balance from runtime meter
	balance, err := programExecuteAction.GetBalance()
	return programActionID, result, balance, err
}
