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
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/utils"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
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
	success, _, output, _, err := programCreateAction.Execute(ctx, nil, db, 0, nil, programID, false)
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
	db *state.SimpleMutable,
	programID ids.ID,
	stepParams []Parameter,
	function string,
	maxUnits uint64,
) (ids.ID, []uint64, error) {
	// create call params from simulation data
	params, err := createCallParams(ctx, db, stepParams)
	if err != nil {
		return ids.Empty, nil, err
	}

	// simulate create program transaction
	programTxID, err := generateRandomID()
	if err != nil {
		return ids.Empty, nil, err
	}

	programExecuteAction := actions.ProgramExecute{
		Function: function,
		Params:   params,
		MaxUnits: maxUnits,
	}

	// execute the action
	success, _, resp, _, err := programExecuteAction.Execute(ctx, nil, db, 0, nil, programTxID, false)
	if !success {
		return ids.Empty, nil, fmt.Errorf("program execution failed: %s", err)
	}
	if err != nil {
		return ids.Empty, nil, err
	}

	p := codec.NewReader(resp, len(resp))
	var result []uint64
	for range resp {
		result = append(result, p.UnpackUint64(true))
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, nil, err
	}

	return programTxID, result, nil
}

func createCallParams(ctx context.Context, db state.Immutable, params []Parameter) ([]runtime.CallParam, error) {
	cp := make([]runtime.CallParam, 0, len(params))
	for _, param := range params {
		switch param.Type {
		case String, ID:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			cp = append(cp, runtime.CallParam{Value: val})
		case Bool:
			val, ok := param.Value.(bool)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			cp = append(cp, runtime.CallParam{Value: boolToUint64(val)})
		case KeyEd25519:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			// get named public key from db
			key, ok, err := storage.GetPublicKey(ctx, db, val)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrNamedKeyNotFound, val)
			}
			if err != nil {
				return nil, err
			}
			cp = append(cp, runtime.CallParam{Value: utils.Address(key)})
		case Uint64:
			switch v := param.Value.(type) {
			case float64:
				// json unmarshal converts to float64
				cp = append(cp, runtime.CallParam{Value: uint64(v)})
			case int:
				if v < 0 {
					return nil, fmt.Errorf("%w: %s", runtime.ErrNegativeValue, param.Type)
				}
				cp = append(cp, runtime.CallParam{Value: uint64(v)})
			default:
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
		default:
			return nil, fmt.Errorf("%w: %s", ErrInvalidParamType, param.Type)
		}
	}

	return cp, nil
}
