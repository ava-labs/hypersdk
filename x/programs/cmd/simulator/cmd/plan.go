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
	"time"

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
	programIDStrMap map[string]string
	stdinReader     io.Reader
}

func newRunCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	r := &runCmd{
		log:             log,
		db:              db,
		programIDStrMap: make(map[string]string),
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
	c.log.Debug("simulation")
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
			zap.Uint64("maxUnits", step.MaxUnits),
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

			now := time.Now().Unix()
			r.Result = Result{
				Msg:       fmt.Sprintf("created key %s", keyName),
				Timestamp: uint64(now),
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
				c.programIDStrMap[fmt.Sprintf("step_%d", i)] = id.String()
				now := time.Now().Unix()
				r.Result = Result{
					ID:        id.String(),
					Timestamp: uint64(now),
				}
				r.Print()
			default:
				r := NewResponse(i)
				if len(step.Params) == 0 {
					return r.Err(fmt.Errorf("%s: %s", ErrInvalidStep.Error(), "execute requires at least 1 param"))
				}

				// get program ID from params
				if step.Params[0].Type != ID {
					return r.Err(fmt.Errorf("%s: %s", ErrInvalidParamType.Error(), step.Params[0].Type))
				}

				idStr, ok := step.Params[0].Value.(string)
				if !ok {
					return r.Err(fmt.Errorf("%s: %s", ErrFailedParamTypeCast.Error(), step.Params[0].Type))
				}

				programIDStr, err := c.verifyProgramIDStr(idStr)
				if err != nil {
					return r.Err(err)
				}
				if idStr != programIDStr {
					step.Params[0].Value = programIDStr
				}

				id, result, err := programExecuteFunc(ctx, c.log, c.db, step.Params, step.Method, step.MaxUnits)
				if err != nil {
					return r.Err(err)
				}

				now := time.Now().Unix()
				if step.Endpoint == ExecuteEndpoint {
					r.Result = Result{
						ID:        id.String(),
						Timestamp: uint64(now),
					}
				} else {
					r.Result = Result{
						Response:  result,
						Timestamp: uint64(now),
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

// verifyProgramIDStr verifies a string is a valid ID and checks the programIDStrMap for
// the synthetic identifier `step_N` where N is the step the id was created from
// execution.
func (r *runCmd) verifyProgramIDStr(idStr string) (string, error) {
	// if the id is valid
	_, err := ids.FromString(idStr)
	if err == nil {
		return idStr, nil
	}

	programIDStr, ok := r.programIDStrMap[idStr]
	if !ok {
		return "", fmt.Errorf("failed to map id: %s", idStr)
	}

	return programIDStr, nil
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
