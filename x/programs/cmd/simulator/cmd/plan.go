// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/utils"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
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
	var planBytes []byte
	if args[0] == "-" {
		// read simulation plan from stdin
		planBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
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

	return nil
}

func (c *runCmd) Verify() error {
	steps := c.plan.Steps
	if steps == nil {
		return fmt.Errorf("%w: %s", ErrInvalidPlan, "no steps found")
	}

	if steps[0].Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidStep, "no params found")
	}

	// verify endpoint requirements
	for i, step := range steps {
		err := verifyEndpoint(i, &step)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifyEndpoint(i int, step *Step) error {
	firstParamType := step.Params[0].Type

	switch step.Endpoint {
	case EndpointKey:
		// verify the first param is a string for key name
		if firstParamType != KeyEd25519 && firstParamType != KeySecp256k1 {
			return fmt.Errorf("%w %d %w: expected ed25519 or secp256k1", ErrInvalidStep, i, ErrInvalidParamType)
		}
	case EndpointReadOnly:
		// verify the first param is an program id
		if firstParamType != ID {
			return fmt.Errorf("%w %d %w: %s", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredID)
		}
	case EndpointExecute:
		if step.Method == ProgramCreate {
			// verify the first param is a string for the path
			if step.Params[0].Type != String {
				return fmt.Errorf("%w %d %w: %s", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredString)
			}
		} else {
			// verify the first param is a program id
			if step.Params[0].Type != ID {
				return fmt.Errorf("%w %d %w: %s", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredID)
			}
		}
	default:
		return fmt.Errorf("%w: %s", ErrInvalidEndpoint, step.Endpoint)
	}
	return nil
}

func (c *runCmd) Run(ctx context.Context) error {
	c.log.Info("simulation",
		zap.String("plan", c.plan.Description),
	)

	for i, step := range c.plan.Steps {
		c.log.Info("simulation",
			zap.Int("step", i),
			zap.String("description", step.Description),
			zap.String("endpoint", string(step.Endpoint)),
			zap.String("method", step.Method),
			zap.Uint64("maxUnits", step.MaxUnits),
			zap.Any("params", step.Params),
		)

		params, err := c.createCallParams(ctx, c.db, step.Params)
		if err != nil {
			return err
		}

		resp := newResponse(i)
		err = runStepFunc(ctx, c.log, c.db, step.Endpoint, step.MaxUnits, step.Method, params, resp)
		if err != nil {
			resp.setError(err)
		}

		// map all transactions to their step_N identifier
		txID, found := resp.getTxID()
		if found {
			c.programIDStrMap[fmt.Sprintf("step_%d", i)] = txID
		}

		// print response to stdout
		err = resp.Print()
		if err != nil {
			return err
		}
	}

	return nil
}

func runStepFunc(
	ctx context.Context,
	log logging.Logger,
	db *state.SimpleMutable,
	endpoint Endpoint,
	maxUnits uint64,
	method string,
	params []runtime.CallParam,
	resp *Response,
) error {
	switch endpoint {
	case EndpointKey:
		keyName := params[0].Value.(string)
		err := keyCreateFunc(ctx, db, keyName)
		if errors.Is(err, ErrDuplicateKeyName) {
			log.Debug("key already exists")
		} else if err != nil {
			return err
		}

		resp.setMsg(fmt.Sprintf("created named key with address %s", keyName))
		resp.setTimestamp(time.Now().Unix())

		return nil
	case EndpointExecute, EndpointReadOnly: // for now the logic is the same for both TODO: breakout readonly
		switch method {
		case ProgramCreate:
			// get program path from params
			programPath := params[0].Value.(string)
			id, err := programCreateFunc(ctx, db, programPath)
			if err != nil {
				return err
			}

			resp.setTxID(id.String())
			resp.setTimestamp(time.Now().Unix())

			return nil
		default:
			id, response, balance, err := programExecuteFunc(ctx, log, db, params, method, maxUnits)
			if err != nil {
				return err
			}

			resp.setTimestamp(time.Now().Unix())
			if endpoint == EndpointExecute {
				resp.setTxID(id.String())
				resp.setBalance(balance)
			} else {
				resp.setResponse(response)
			}
			return nil
		}
	}
	return nil
}

// createCallParams converts a slice of Parameters to a slice of runtime.CallParams.
func (c *runCmd) createCallParams(ctx context.Context, db state.Immutable, params []Parameter) ([]runtime.CallParam, error) {
	cp := make([]runtime.CallParam, 0, len(params))
	for _, param := range params {
		switch param.Type {
		case String, ID:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			val, err := c.verifyProgramIDStr(val)
			if err != nil {
				return nil, err
			}
			cp = append(cp, runtime.CallParam{Value: val})
		case Bool:
			val, ok := param.Value.(bool)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			cp = append(cp, runtime.CallParam{Value: boolToUint64(val)})
		case KeyEd25519: // TODO: support secp256k1
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

// verifyProgramIDStr verifies a string is a valid ID and checks the programIDStrMap for
// the synthetic identifier `step_N` where N is the step the id was created from
// execution.
func (r *runCmd) verifyProgramIDStr(idStr string) (string, error) {
	// if the id is valid
	_, err := ids.FromString(idStr)
	if err == nil {
		return idStr, nil
	}

	// check if the id is a synthetic identifier
	if strings.HasPrefix(idStr, "step_") {
		stepID, ok := r.programIDStrMap[idStr]
		if !ok {
			return "", fmt.Errorf("failed to map to id: %s", idStr)
		}
		return stepID, nil
	}

	return idStr, nil
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
