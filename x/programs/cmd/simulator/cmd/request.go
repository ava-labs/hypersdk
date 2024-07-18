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

	// counts the number of requests made
	numRequests *int
	// the message send to the simulator
	requestMessage *string
	// the simulator request
	request *SimulatorRequest

	log    logging.Logger
	reader *bufio.Reader

	// tracks program IDs created during this simulation
	programIDStrMap map[int]codec.Address
}

func (c *runCmd) New(parser *argparse.Parser, programIDStrMap map[int]codec.Address, lastRequest *int, reader *bufio.Reader) {
	c.programIDStrMap = programIDStrMap
	c.cmd = parser.NewCommand("run", "Run a HyperSDK program simulation plan")

	c.requestMessage = c.cmd.String("", "message", &argparse.Options{
		Required: true,
	})
	c.numRequests = lastRequest
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
	resp, err := c.RunRequest(ctx, db)
	if err != nil {
		return newResponse(0), err
	}
	return resp, nil
}

func (c *runCmd) Happened() bool {
	return c.cmd.Happened()
}

func (c *runCmd) Init() (err error) {
	var marshaledBytes []byte
	if c.requestMessage == nil || *c.requestMessage == "" {
		return errors.New("please specify either a valid run command")
	}

	marshaledBytes = []byte(*c.requestMessage)
	c.request, err = unmarshalRequest(marshaledBytes)
	if err != nil {
		return err
	}

	return nil
}

func (c *runCmd) Verify() error {
	request := c.request
	if request == nil {
		return fmt.Errorf("%w: %s", ErrInvalidRequest, "no request found")
	}

	if request.Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidParams, "no params found")
	}

	// verify endpoint requirements
	return verifyEndpoint(*c.numRequests, request)
}

func verifyEndpoint(i int, request *SimulatorRequest) error {
	firstParamType := request.Params[0].Type

	switch request.Endpoint {
	case EndpointReadOnly:
		// verify the first param is a test context
		if firstParamType != TestContext {
			return fmt.Errorf("%w %d %w: %w", ErrInvalidRequest, i, ErrInvalidParamType, ErrFirstParamRequiredContext)
		}
	case EndpointExecute:
		// verify the first param is a test context
		if request.Params[0].Type != TestContext {
			return fmt.Errorf("%w %d %w: %w", ErrInvalidRequest, i, ErrInvalidParamType, ErrFirstParamRequiredContext)
		}
	case EndpointCreateProgram:
		// verify the first param is a string for the path
		if request.Params[0].Type != Path {
			return fmt.Errorf("%w %d %w: %w", ErrInvalidRequest, i, ErrInvalidParamType, ErrFirstParamRequiredPath)
		}
	default:
		return fmt.Errorf("%w: %s", ErrInvalidEndpoint, request.Endpoint)
	}
	return nil
}

func (c *runCmd) RunRequest(ctx context.Context, db *state.SimpleMutable) (*Response, error) {
	index := *c.numRequests
	request := c.request
	c.log.Info("simulation",
		zap.Int("request", index),
		zap.String("endpoint", string(request.Endpoint)),
		zap.String("method", request.Method),
		zap.Uint64("maxUnits", request.MaxUnits),
		zap.Any("params", request.Params),
	)

	params, err := c.createCallParams(request.Params)
	if err != nil {
		c.log.Error(fmt.Sprintf("simulation call: %s", err))
		return newResponse(0), err
	}

	resp := newResponse(index)
	err = c.runRequestFunc(ctx, db, request.Endpoint, request.MaxUnits, request.Method, params, resp)
	if err != nil {
		resp.setError(err)
	}

	*c.numRequests = index + 1

	return resp, nil
}

// runRequestFunc runs the simulation request and returns the response.
func (c *runCmd) runRequestFunc(
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
	case EndpointExecute: // for now the logic is the same for both Execute & ReadOnly: breakout readonly
		var simulatorTestContext SimulatorTestContext
		err := json.Unmarshal(params[0].Value, &simulatorTestContext)
		if err != nil {
			return err
		}
		program, err := simulatorTestContext.Program(c.programIDStrMap)
		if err != nil {
			return err
		}
		testContext := runtime.Context{
			Program:   program,
			Actor:     simulatorTestContext.ActorAddr,
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
		testContext := runtime.Context{
			Program:   program,
			Actor:     simulatorTestContext.ActorAddr,
			Timestamp: simulatorTestContext.Timestamp,
			Height:    simulatorTestContext.Height,
		}

		// TODO: implement readonly for now just don't charge for gas
		result, _, err := programExecuteFunc(ctx, c.log, db, testContext, params[1:], method, math.MaxUint64)
		output := resultToOutput(result, err)
		// check if the db has any new changes
		if db.HasChanges() {
			changes := db.Changes()
			return fmt.Errorf("%w: %s, %s", ErrInvalidRequest, "readonly request should not have any changes", changes)
		}
		if err := db.Commit(ctx); err != nil {
			return err
		}
		response, err := runtime.Serialize(output)
		if err != nil {
			return err
		}

		resp.setResponse(response)

		return nil
	case EndpointCreateProgram:
		// get program path from params
		programPath := string(params[0].Value)
		programAddress, err := programCreateFunc(ctx, db, programPath)
		if err != nil {
			return err
		}
		c.programIDStrMap[*c.numRequests] = programAddress
		resp.setTimestamp(time.Now().Unix())

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
	ProgramID uint64        `json:"programId"`
	ActorAddr codec.Address `json:"actor"`
	Height    uint64        `json:"height"`
	Timestamp uint64        `json:"timestamp"`
}

func (s *SimulatorTestContext) Program(programIDStrMap map[int]codec.Address) (codec.Address, error) {
	id := s.ProgramID
	programAddress, ok := programIDStrMap[int(id)]
	if !ok {
		return codec.EmptyAddress, fmt.Errorf("failed to map to id: %d", id)
	}
	return programAddress, nil
}

func AddressToString(pk ed25519.PublicKey) string {
	addrString, _ := address.FormatBech32("matrix", pk[:])
	return addrString
}

// createCallParams converts a slice of Parameters to a slice of runtime.CallParams.
func (c *runCmd) createCallParams(params []Parameter) ([]Parameter, error) {
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
