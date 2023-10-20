// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
)

type runCmd struct {
	plan *Plan
	log  logging.Logger
	db   *state.SimpleMutable

	// tracks program IDs created during this simulation
	programMap  map[string]ids.ID
	stdinReader io.Reader
}

func newRunCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	r := &runCmd{
		log:        log,
		db:         db,
		programMap: make(map[string]ids.ID),
	}
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run a HyperSDK program simulation plan",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// if the first argument is "-" read from stdin
			r.stdinReader = cmd.InOrStdin()
			err := r.Init(args)
			if err != nil {
				return err
			}
			err = r.Verify()
			if err != nil {
				return err
			}
			return r.Run(cmd.Context())
		},
	}

	return cmd
}

func (c *runCmd) Init(args []string) (err error) {
	var planBytes []byte
	if args[0] == "-" {
		// read simulation plan from stdin
		reader := bufio.NewReader(os.Stdin)
		planStr, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		planBytes = []byte(planStr)
	} else {
		// read simulation plan from arg[0]
		planBytes, err = os.ReadFile(args[0])
		if err != nil {
			return err
		}
	}

	c.plan, err = unmarshalPlan(planBytes)
	if err != nil {
		return err
	}

	if c.plan.Steps == nil {
		return fmt.Errorf("%w: %s", ErrInvalidPlan, "no steps found")
	}

	if c.plan.Steps[0].Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidStep, "no params found")
	}

	return nil
}

func (r *runCmd) Verify() error {
	return nil
}

func (c *runCmd) Run(ctx context.Context) error {
	c.log.Debug("simulation",
		zap.String("plan", c.plan.Name),
	)

	for i, step := range c.plan.Steps {
		c.log.Debug("simulation",
			zap.Int("step", i),
			zap.String("description", step.Description),
			zap.String("endpoint", string(step.Endpoint)),
			zap.String("method", step.Method),
			zap.Any("params", step.Params),
		)

		switch step.Endpoint {
		case KeyEndpoint:
			r := NewResponse(i)
			keyName, ok := step.Params[0].Value.(string)
			if !ok {
				return r.Err(fmt.Errorf("%w: %s", ErrFailedParamTypeCast, step.Params[0].Type))
			}

			err := keyCreateFunc(ctx, c.db, keyName)
			if errors.Is(err, ErrDuplicateKeyName) {
				c.log.Debug("key already exists")
			} else if err != nil {
				return r.Err(err)
			}
			r.Result = Result{
				Msg: fmt.Sprintf("created key %s", keyName),
			}
			r.Print()
		case ExecuteEndpoint, ReadOnlyEndpoint: // for now the logic is the same for both
			switch step.Method {
			case ProgramCreate:
				r := NewResponse(i)
				// get program path from params
				programPath, ok := step.Params[0].Value.(string)
				if !ok {
					return r.Err(fmt.Errorf("%x: %s", ErrFailedParamTypeCast.Error(), step.Params[0].Type))
				}
				id, err := programCreateFunc(ctx, c.db, programPath)
				if err != nil {
					return r.Err(err)
				}
				// create a mapping from the step id to the program id for use
				// during inline program executions.
				c.programMap[fmt.Sprintf("step_%d", i)] = id
				r.Result = Result{
					ID: id.String(),
				}
				r.Print()
			default:
				r := NewResponse(i)
				if len(step.Params) < 2 {
					return r.Err(fmt.Errorf("%s: %s", ErrInvalidStep.Error(), "execute requires at least 2 params"))
				}

				// get program ID from params
				if step.Params[0].Type != ID {
					return r.Err(fmt.Errorf("%s: %s", ErrInvalidParamType.Error(), step.Params[0].Type))
				}
				idStr, ok := step.Params[0].Value.(string)
				if !ok {
					return r.Err(fmt.Errorf("%s: %s", ErrFailedParamTypeCast.Error(), step.Params[0].Type))
				}
				programID, err := c.getProgramID(idStr)
				if err != nil {
					return r.Err(err)
				}

				// maxUnits from params
				if step.Params[1].Type != Uint64 {
					return r.Err(fmt.Errorf("%s: %s", ErrInvalidParamType.Error(), step.Params[1].Type))
				}
				maxUnits, err := intToUint64(step.Params[1].Value)
				if err != nil {
					return r.Err(err)
				}

				id, result, err := programExecuteFunc(ctx, c.db, programID, step.Params, step.Method, maxUnits)
				if err != nil {
					return r.Err(err)
				}

				if step.Method == ProgramExecute {
					r.Result = Result{
						ID:      id.String(),
						Balance: result[0],
					}
				} else {
					r.Result = Result{
						Response: result,
					}
				}
				r.Print()
			}

		default:
			return NewResponse(i).Err(fmt.Errorf("%w: %s", ErrInvalidEndpoint, step.Endpoint))
		}
	}
	return nil
}

// getProgramID checks the program map for the synthetic identifier `step_N`
// where N is the step the id was created from execution.
func (r *runCmd) getProgramID(idStr string) (ids.ID, error) {
	if r.programMap[idStr] != ids.Empty {
		programID, ok := r.programMap[idStr]
		if ok {
			return programID, nil
		}
	}

	return ids.FromString(idStr)
}

// generateRandomID creates a unique ID.
// Note: ids.GenerateID() is not used because the IDs are not unique and will
// collide.
func generateRandomID() (ids.ID, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return ids.Empty, err
	}
	id, err := ids.ToID(key)
	if err != nil {
		return ids.Empty, err
	}

	return id, nil
}
