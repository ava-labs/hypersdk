// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/actions"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/utils"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

var _ Cmd = (*runCmd)(nil)

type runCmd struct {
	cmd *argparse.Command

	lastStep *int
	file     *string
	planStep *string

	step   *Step
	log    logging.Logger
	reader *bufio.Reader

	// tracks program IDs created during this simulation
	programIDStrMap map[string]string
}

func (c *runCmd) New(parser *argparse.Parser, programIDStrMap map[string]string, lastStep *int, reader *bufio.Reader) {
	c.programIDStrMap = programIDStrMap
	c.cmd = parser.NewCommand("run", "Run a HyperSDK program simulation plan")
	c.file = c.cmd.String("", "file", &argparse.Options{
		Required: false,
	})
	c.planStep = c.cmd.String("", "step", &argparse.Options{
		Required: false,
	})
	c.lastStep = lastStep
	c.reader = reader
}

func (c *runCmd) Run(ctx context.Context, log logging.Logger, db *state.SimpleMutable, args []string) (*Response, error) {
	c.log = log
	var err error
	if err = c.Init(); err != nil {
		return newResponse(0), err
	}
	if err = c.Verify(); err != nil {
		return newResponse(0), err
	}
	resp, err := c.RunStep(ctx, db)
	if err != nil {
		return newResponse(0), err
	}
	return resp, nil
}

func (c *runCmd) Happened() bool {
	return c.cmd.Happened()
}

func (c *runCmd) Init() (err error) {
	var planStep []byte
	if c.planStep != nil && len(*c.planStep) > 0 {
		planStep = []byte(*c.planStep)
	} else if len(*c.file) > 0 {
		// read simulation step from file
		planStep, err = os.ReadFile(*c.file)
		if err != nil {
			return err
		}
	} else {
		return errors.New("please specify either a --plan or a --file flag")
	}

	c.step, err = unmarshalStep(planStep)
	if err != nil {
		return err
	}

	return nil
}

func (c *runCmd) Verify() error {
	step := c.step
	if step == nil {
		return fmt.Errorf("%w: %s", ErrInvalidPlan, "no steps found")
	}

	if step.Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidStep, "no params found")
	}

	// verify endpoint requirements
	err := verifyEndpoint(*c.lastStep, step)
	if err != nil {
		return err
	}

	// verify assertions
	if step.Require != nil {
		err = verifyAssertion(*c.lastStep, step.Require)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifyAssertion(i int, require *Require) error {
	if require == nil {
		return nil
	}
	if require.Result.Operator == "" {
		return fmt.Errorf("%w %d: missing assertion operator", ErrInvalidStep, i)
	}
	if require.Result.Value == "" {
		return fmt.Errorf("%w %d: missing assertion value", ErrInvalidStep, i)
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
		// verify the first param is a program ID
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

func (c *runCmd) RunStep(ctx context.Context, db *state.SimpleMutable) (*Response, error) {
	index := *c.lastStep
	step := c.step
	c.log.Info("simulation",
		zap.Int("step", index),
		zap.String("endpoint", string(step.Endpoint)),
		zap.String("method", step.Method),
		zap.Uint64("maxUnits", step.MaxUnits),
		zap.Any("params", step.Params),
	)

	params, err := c.createCallParams(ctx, db, step.Params)
	if err != nil {
		c.log.Error("simulation call", zap.Error(err))
		return newResponse(0), err
	}

	resp := newResponse(index)
	err = runStepFunc(ctx, c.log, db, step.Endpoint, step.MaxUnits, step.Method, params, step.Require, resp)
	if err != nil {
		c.log.Debug("simulation step", zap.Error(err))
		resp.setError(err)
	}

	// map all transactions to their step_N identifier
	txID, found := resp.getTxID()
	if found {
		c.programIDStrMap[fmt.Sprintf("step_%d", index)] = txID
	}

	lastStep := index + 1
	*c.lastStep = lastStep

	return resp, nil
}

func runStepFunc(
	ctx context.Context,
	log logging.Logger,
	db *state.SimpleMutable,
	endpoint Endpoint,
	maxUnits uint64,
	method string,
	params []actions.CallParam,
	require *Require,
	resp *Response,
) error {
	defer resp.setTimestamp(time.Now().Unix())
	switch endpoint {
	case EndpointKey:
		keyName := params[0].Value.(string)
		key, err := keyCreateFunc(ctx, db, keyName)
		if errors.Is(err, ErrDuplicateKeyName) {
			log.Debug("key already exists")
		} else if err != nil {
			return err
		}
		resp.setMsg(fmt.Sprintf("created named key with address %s", utils.Address(key)))

		return nil
	case EndpointExecute: // for now the logic is the same for both TODO: breakout readonly
		if method == ProgramCreate {
			// get program path from params
			programPath := params[0].Value.(string)
			id, err := programCreateFunc(ctx, db, programPath)
			if err != nil {
				return err
			}
			resp.setTxID(id.String())
			resp.setTimestamp(time.Now().Unix())

			return nil
		}
		id, response, balance, err := programExecuteFunc(ctx, log, db, params, method, maxUnits)
		if err != nil {
			return err
		}
		resp.setResponse(response)
		ok, err := validateAssertion(response, require)
		if !ok {
			return fmt.Errorf("%w", ErrResultAssertionFailed)
		}
		if err != nil {
			return err
		}
		resp.setTxID(id.String())
		resp.setBalance(balance)

		return nil
	case EndpointReadOnly:
		// TODO: implement readonly for now just don't charge for gas
		_, response, _, err := programExecuteFunc(ctx, log, db, params, method, math.MaxUint64)
		if err != nil {
			return err
		}

		resp.setResponse(response)
		ok, err := validateAssertion(response, require)

		if !ok {
			return fmt.Errorf("%w", ErrResultAssertionFailed)
		}
		if err != nil {
			return err
		}

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidEndpoint, endpoint)
	}
}

// createCallParams converts a slice of Parameters to a slice of runtime.CallParams.
func (c *runCmd) createCallParams(ctx context.Context, db state.Immutable, params []Parameter) ([]actions.CallParam, error) {
	cp := make([]actions.CallParam, 0, len(params))
	for _, param := range params {
		switch param.Type {
		case String, ID:
			stepIdStr, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			if strings.HasPrefix(stepIdStr, "step_") {
				programIdStr, ok := c.programIDStrMap[stepIdStr]
				if !ok {
					return nil, fmt.Errorf("failed to map to id: %s", stepIdStr)
				}
				programId, err := ids.FromString(programIdStr)
				if err != nil {
					return nil, err
				}
				cp = append(cp, actions.CallParam{Value: programId})
			} else {
				programId, err := ids.FromString(stepIdStr)
				if err == nil {
					cp = append(cp, actions.CallParam{Value: programId})
				} else {
					// this is a path to the wasm program
					cp = append(cp, actions.CallParam{Value: stepIdStr})
				}
			}
		case Bool:
			val, ok := param.Value.(bool)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			cp = append(cp, actions.CallParam{Value: boolToUint64(val)})
		case KeyEd25519: // TODO: support secp256k1
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}

			key := val
			// get named public key from db
			pk, ok, err := storage.GetPublicKey(ctx, db, val)
			if ok {
				// otherwise use the public key address
				key = string(pk[:])
			}
			if err != nil {
				return nil, err
			}
			cp = append(cp, actions.CallParam{Value: key})
		case Uint64:
			switch v := param.Value.(type) {
			case float64:
				// json unmarshal converts to float64
				cp = append(cp, actions.CallParam{Value: uint64(v)})
			case int:
				if v < 0 {
					return nil, fmt.Errorf("%w: %s", program.ErrNegativeValue, param.Type)
				}
				cp = append(cp, actions.CallParam{Value: uint64(v)})
			case string:
				number, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
				}
				cp = append(cp, actions.CallParam{Value: number})
			default:
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
		default:
			return nil, fmt.Errorf("%w: %s", ErrInvalidParamType, param.Type)
		}
	}

	return cp, nil
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
