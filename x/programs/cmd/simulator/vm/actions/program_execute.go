// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"fmt"

	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	importProgram "github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	Function string      `json:"programFunction"`
	MaxUnits uint64      `json:"maxUnits"`
	Params   []CallParam `json:"params"`

	Log logging.Logger

	rt runtime.Runtime
}

func (*ProgramExecute) GetTypeID() uint8 {
	return programExecuteID
}

func (t *ProgramExecute) StateKeys(actor codec.Address, txID ids.ID) state.Keys {
	return state.Keys{}
}

func (*ProgramExecute) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.ProgramChunks}
}

func (t *ProgramExecute) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (success bool, computeUnits uint64, output []byte, err error) {
	if len(t.Function) == 0 {
		return false, 1, OutputValueZero, nil
	}
	if len(t.Params) == 0 {
		return false, 1, OutputValueZero, nil
	}

	programID, ok := t.Params[0].Value.(ids.ID)
	if !ok {
		return false, 1, utils.ErrBytes(fmt.Errorf("invalid call param: must be ID")), nil
	}

	// TODO: take fee out of balance?
	programBytes, exists, err := storage.GetProgram(context.Background(), mu, programID)
	if !exists {
		err = errors.New("unknown program")
	}
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}

	// TODO: get cfg from genesis
	cfg := runtime.NewConfig()
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}

	ecfg, err := engine.NewConfigBuilder().
		WithDefaultCache(true).
		Build()
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}
	eng := engine.New(ecfg)

	// TODO: allow configurable imports?
	importsBuilder := host.NewImportsBuilder()
	importsBuilder.Register("state", func() host.Import {
		return pstate.New(logging.NoLog{}, mu)
	})
	callContext := &program.Context{
		ProgramID: programID,
		// Actor:            [32]byte(actor[1:]),
		// OriginatingActor: [32]byte(actor[1:])
	}

	importsBuilder.Register("program", func() host.Import {
		return importProgram.New(logging.NoLog{}, eng, mu, cfg, callContext)
	})
	imports := importsBuilder.Build()

	t.rt = runtime.New(logging.NoLog{}, eng, imports, cfg)
	err = t.rt.Initialize(ctx, callContext, programBytes, t.MaxUnits)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}
	defer t.rt.Stop()

	mem, err := t.rt.Memory()
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}
	params, err := WriteParams(mem, t.Params)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}

	resp, err := t.rt.Call(ctx, t.Function, callContext, params[1:]...)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil
	}

	return true, 1, resp, nil
}

func (*ProgramExecute) MaxComputeUnits(chain.Rules) uint64 {
	return ProgramExecuteComputeUnits
}

func (*ProgramExecute) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (t *ProgramExecute) Marshal(p *codec.Packer) {
	// TODO
}

func (t *ProgramExecute) GetBalance() (uint64, error) {
	return t.rt.Meter().GetBalance()
}

func UnmarshalProgramExecute(p *codec.Packer) (chain.Action, error) {
	// TODO
	return nil, nil
}

func (*ProgramExecute) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// CallParam defines a value to be passed to a guest function.
type CallParam struct {
	Value interface{} `json,yaml:"value"`
}

// WriteParams is a helper function that writes the given params to memory if non integer.
// Supported types include int, uint64 and string.
func WriteParams(m *program.Memory, p []CallParam) ([]uint32, error) {
	var params []uint32
	for _, param := range p {
		switch v := param.Value.(type) {
		case []byte:
			smartPtr, err := program.AllocateBytes(v, m)
			if err != nil {
				return nil, err
			}
			params = append(params, smartPtr)
		case ids.ID:
			smartPtr, err := program.AllocateBytes(v[:], m)
			if err != nil {
				return nil, err
			}
			params = append(params, smartPtr)
		case string:
			smartPtr, err := program.AllocateBytes([]byte(v), m)
			if err != nil {
				return nil, err
			}
			params = append(params, smartPtr)
		case uint32:
			params = append(params, v)
		default:
			ptr, err := writeToMem(v, m)
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		}
	}

	return params, nil
}

// SerializeParameter serializes [obj] using Borsh
func serializeParameter(obj interface{}) ([]byte, error) {
	bytes, err := borsh.Serialize(obj)
	return bytes, err
}

// Serialize the parameter and create a smart ptr
func writeToMem(obj interface{}, memory *program.Memory) (uint32, error) {
	bytes, err := serializeParameter(obj)
	if err != nil {
		return 0, err
	}

	return program.AllocateBytes(bytes, memory)
}
