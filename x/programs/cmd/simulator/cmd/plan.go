// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
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
	programIDStrMap map[int]codec.Address
}

func (c *runCmd) New(parser *argparse.Parser, programIDStrMap map[int]codec.Address, lastStep *int, reader *bufio.Reader) {
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

func (c *runCmd) Run(ctx context.Context, log logging.Logger, db *state.SimpleMutable, _ []string) (*Response, error) {
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
	switch {
	case c.planStep != nil && len(*c.planStep) > 0:
		{
			planStep = []byte(*c.planStep)
		}
	case len(*c.file) > 0:
		{
			// read simulation step from file
			planStep, err = os.ReadFile(*c.file)
			if err != nil {
				return err
			}
		}
	default:
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
		return fmt.Errorf("%w: %s", ErrInvalidStep, "no steps found")
	}

	if step.Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidParams, "no params found")
	}

	// verify endpoint requirements
	return verifyEndpoint(*c.lastStep, step)
}

func verifyEndpoint(i int, step *Step) error {
	firstParamType := step.Params[0].Type

	switch step.Endpoint {
	case EndpointKey:
		if step.Method == KeyCreate {
			// verify the first param is a string for key name
			if firstParamType != KeyEd25519 && firstParamType != KeySecp256k1 {
				return fmt.Errorf("%w %d %w: expected ed25519 or secp256k1", ErrInvalidStep, i, ErrInvalidParamType)
			}
		} else {
			return fmt.Errorf("%w: %s", ErrInvalidMethod, step.Method)
		}
	case EndpointReadOnly:
		// verify the first param is a test context
		if firstParamType != TestContext {
			return fmt.Errorf("%w %d %w: %w", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredContext)
		}
	case EndpointExecute:
		if step.Method == ProgramCreate {
			// verify the first param is a string for the path
			if step.Params[0].Type != Path {
				return fmt.Errorf("%w %d %w: %w", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredPath)
			}
		} else {
			// verify the first param is a test context
			if step.Params[0].Type != TestContext {
				return fmt.Errorf("%w %d %w: %w", ErrInvalidStep, i, ErrInvalidParamType, ErrFirstParamRequiredContext)
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

	params, err := c.createCallParams(ctx, db, step.Params, step.Endpoint)
	if err != nil {
		c.log.Error(fmt.Sprintf("simulation call: %s", err))
		return newResponse(0), err
	}

	resp := newResponse(index)
	err = c.runStepFunc(ctx, db, step.Endpoint, step.MaxUnits, step.Method, params, resp)
	if err != nil {
		resp.setError(err)
	}

	lastStep := index + 1
	*c.lastStep = lastStep

	return resp, nil
}

func (c *runCmd) runStepFunc(
	ctx context.Context,
	db *state.SimpleMutable,
	endpoint Endpoint,
	maxUnits uint64,
	method string,
	params []Parameter,
	resp *Response,
) error {
	defer resp.setTimestamp(time.Now().Unix())
	switch endpoint {
	case EndpointKey:
		keyName := string(params[0].Value)
		key, err := keyCreateFunc(ctx, db, keyName)
		if errors.Is(err, ErrDuplicateKeyName) {
			c.log.Debug("key already exists")
		} else if err != nil {
			return err
		}
		resp.setMsg("created named key with address " + AddressToString(key))

		return nil
	case EndpointExecute: // for now the logic is the same for both TODO: breakout readonly
		if method == ProgramCreate {
			// get program path from params
			programPath := string(params[0].Value)
			programAddress, err := programCreateFunc(ctx, db, programPath)
			if err != nil {
				return err
			}
			c.programIDStrMap[*c.lastStep] = programAddress
			resp.setTimestamp(time.Now().Unix())

			return nil
		}

		var simulatorTestContext SimulatorTestContext
		err := json.Unmarshal(params[0].Value, &simulatorTestContext)
		if err != nil {
			return err
		}
		program, err := simulatorTestContext.Program(c.programIDStrMap)
		if err != nil {
			return err
		}
		actor, err := simulatorTestContext.Actor(ctx, db)
		if err != nil {
			return err
		}
		testContext := runtime.Context{
			Program:   program,
			Actor:     actor,
			Timestamp: simulatorTestContext.Timestamp,
			Height:    simulatorTestContext.Height,
		}

		result, balance, err := programExecuteFunc(ctx, c.log, db, testContext, params[1:], method, maxUnits)
		output := resultToOutput(result, err)
		if err := db.Commit(ctx); err != nil {
			return err
		}
		response, err := runtime.Serialize(output)
		if err != nil {
			return err
		}

		resp.setResponse(response)
		resp.setBalance(balance)

		return nil
	case EndpointReadOnly:
		var simulatorTestContext SimulatorTestContext
		err := json.Unmarshal(params[0].Value, &simulatorTestContext)
		if err != nil {
			return err
		}
		program, err := simulatorTestContext.Program(c.programIDStrMap)
		if err != nil {
			return err
		}
		actor, err := simulatorTestContext.Actor(ctx, db)
		if err != nil {
			return err
		}
		testContext := runtime.Context{
			Program:   program,
			Actor:     actor,
			Timestamp: simulatorTestContext.Timestamp,
			Height:    simulatorTestContext.Height,
		}

		// TODO: implement readonly for now just don't charge for gas
		result, _, err := programExecuteFunc(ctx, c.log, db, testContext, params[1:], method, math.MaxUint64)
		output := resultToOutput(result, err)
		if err := db.Commit(ctx); err != nil {
			return err
		}
		response, err := runtime.Serialize(output)
		if err != nil {
			return err
		}

		resp.setResponse(response)

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidEndpoint, endpoint)
	}
}

func resultToOutput(result []byte, err error) runtime.Result[runtime.RawBytes, runtime.ProgramCallErrorCode] {
	if err != nil {
		if code, ok := runtime.ExtractProgramCallErrorCode(err); ok {
			return runtime.Err[runtime.RawBytes, runtime.ProgramCallErrorCode](code)
		}
		return runtime.Err[runtime.RawBytes, runtime.ProgramCallErrorCode](runtime.ExecutionFailure)
	}

	return runtime.Ok[runtime.RawBytes, runtime.ProgramCallErrorCode](result)
}

type SimulatorTestContext struct {
	ProgramID uint64     `json:"programId"`
	ActorKey  *Parameter `json:"actorKey"`
	Height    uint64     `json:"height"`
	Timestamp uint64     `json:"timestamp"`
}

func (s *SimulatorTestContext) Program(programIDStrMap map[int]codec.Address) (codec.Address, error) {
	id := s.ProgramID
	programAddress, ok := programIDStrMap[int(id)]
	if !ok {
		return codec.EmptyAddress, fmt.Errorf("failed to map to id: %d", id)
	}
	return programAddress, nil
}

func (s *SimulatorTestContext) Actor(ctx context.Context, db *state.SimpleMutable) (codec.Address, error) {
	actor := codec.EmptyAddress
	if s.ActorKey != nil {
		key := string(s.ActorKey.Value)
		pk, ok, err := GetPublicKey(ctx, db, key)
		if err != nil {
			return codec.EmptyAddress, err
		}
		if !ok {
			return codec.EmptyAddress, fmt.Errorf("%w: %s", ErrNamedKeyNotFound, key)
		}
		id, err := ids.ToID(pk[:])
		if err != nil {
			return codec.EmptyAddress, err
		}
		actor = codec.CreateAddress(0, id)
	}
	return actor, nil
}

func AddressToString(pk ed25519.PublicKey) string {
	addrString, _ := address.FormatBech32("matrix", pk[:])
	return addrString
}

// createCallParams converts a slice of Parameters to a slice of runtime.CallParams.
func (c *runCmd) createCallParams(ctx context.Context, db state.Immutable, params []Parameter, endpoint Endpoint) ([]Parameter, error) {
	cp := make([]Parameter, 0, len(params))
	for _, param := range params {
		switch param.Type {
		case ID:
			if len(param.Value) != 8 {
				return nil, fmt.Errorf("invalid length %d for id", len(param.Value))
			}
			id := binary.LittleEndian.Uint64(param.Value)
			programAddress, ok := c.programIDStrMap[int(id)]
			if !ok {
				return nil, fmt.Errorf("failed to map to id: %d", id)
			}
			cp = append(cp, Parameter{Value: programAddress[:], Type: param.Type})
		case Path:
			path := string(param.Value)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("this path does not exists")
			}
			cp = append(cp, param)
		case Address:
			programAddress, err := codec.ToAddress(param.Value)
			if err != nil {
				return nil, errors.New("invalid address")
			}
			cp = append(cp, Parameter{Value: programAddress[:], Type: param.Type})
		case KeyEd25519: // TODO: support secp256k1
			key := param.Value
			// get named public key from db
			pk, ok, err := GetPublicKey(ctx, db, string(param.Value))
			if err != nil {
				return nil, err
			}
			if !ok && endpoint != EndpointKey {
				// using not stored named public key in other context than key creation
				return nil, fmt.Errorf("%w: %s", ErrNamedKeyNotFound, string(param.Value))
			}
			if ok {
				id, err := ids.ToID(pk[:])
				if err != nil {
					return nil, err
				}
				address := codec.CreateAddress(0, id)
				key = address[:]
			}
			cp = append(cp, Parameter{Value: key, Type: param.Type})
		default:
			cp = append(cp, param)
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
