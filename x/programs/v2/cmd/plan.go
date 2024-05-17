// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/akamensky/argparse"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"

	// "gopkg.in/yaml.v2"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
)

const (
	Key    = "key"
	Call  = "call"
	Create  = "create"
)

var _ Cmd = (*runCmd)(nil)

type runCmd struct {
	cmd *argparse.Command

	lastStep *int
	file     *string
	planStep *string

	step		 Operation
	log    logging.Logger
	reader *bufio.Reader

	// tracks program IDs created during this simulation
	// TODO #[deprecated]
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

func unmarshalStep(bytes []byte) (Operation, error) {
	// TODO put back yaml
	var rawStep RawStep
	err := json.Unmarshal(bytes, &rawStep)
	if err != nil {
		return nil, err
	}
	msg := rawStep.Message[rawStep.StepType]
	fmt.Fprintln(os.Stderr, msg)

	switch rawStep.StepType {
	case Key:
		var s *KeyStep
		if err := mapstructure.Decode(msg, &s); err != nil {
			return nil, err
		}
		fmt.Fprintln(os.Stderr, s)
		return s, nil
	case Call:
		var s *CallStep
		if err := mapstructure.Decode(msg, &s); err != nil {
			return nil, err
		}
		return s, nil
	case Create:
		var s *CreateStep
		if err := mapstructure.Decode(msg, &s); err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, errors.New("unsupported step type")
	}

	// TODO conditionally deserialize the message given the message type
	// this should be platform agnostic btw!
	// if false {
	// switch {
	// case isJSON(string(bytes)):
	// 	if err := json.Unmarshal(bytes, &s); err != nil {
	// 		return nil, err
	// 	}
	// case isYAML(string(bytes)):
	// 	if err := yaml.Unmarshal(bytes, &s); err != nil {
	// 		return nil, err
	// 	}
	// default:
	// 	// TODO
	// 	// return nil, ErrInvalidConfigFormat
	// 	return nil, nil
	// }
// }

	// return s, nil
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
		// return fmt.Errorf("%w: %s", ErrInvalidPlan, "no steps found")
		return nil
	}

	switch v := step.(type) {
	case *CallStep:
		if v.Require != nil {
			err := verifyAssertion(*c.lastStep, v.Require)
			if err != nil {
				return err
			}
		}
	case *KeyStep:
		fmt.Fprintln(os.Stderr, v.Curve)
		if v.Curve != KeyEd25519 && v.Curve != KeySecp256k1 {
			return fmt.Errorf("%w %d %w: expected ed25519 or secp256k1", ErrInvalidStep, *c.lastStep, ErrInvalidParamType)
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

func (c *runCmd) RunStep(ctx context.Context, db *state.SimpleMutable) (*Response, error) {
	index := *c.lastStep
	step := c.step
	// TODO most of these are not really relevant anymore
	// c.log.Info("simulation",
	// 	zap.Int("step", index),
	// 	zap.String("endpoint", string(step.Endpoint)),
	// 	zap.String("method", step.Method),
	// 	zap.Uint64("maxUnits", step.MaxUnits),
	// 	zap.Any("params", step.Params),
	// )

	resp := newResponse(index)
	defer resp.SetTimestamp(time.Now().Unix())
	err := step.Execute(ctx, c.log, db, resp)
	if err != nil {
		c.log.Debug("simulation step", zap.Error(err))
		resp.SetError(err)
	}

	// TODO I doubt this is extremely useful, we can just use the program ID
	// map all transactions to their step_N identifier
	// txID, found := resp.GetTxID()
	// if found {
	// 	c.programIDStrMap[fmt.Sprintf("step_%d", index)] = txID
	// }

	lastStep := index + 1
	*c.lastStep = lastStep

	return resp, nil
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
