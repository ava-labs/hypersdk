// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"

	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/program"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"

	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/v2/runtime"
)

type Type string

const (
	String       Type = "string"
	Bool         Type = "bool"
	ID           Type = "id"
	Uint64       Type = "u64"
)

type Parameter struct {
	// The type of the parameter. (required)
	Type Type `json,yaml:"type"`
	// The value of the parameter. (required)
	Value interface{} `json,yaml:"value"`
}

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	Function  string      `json:"programFunction"`
	MaxUnits  uint64      `json:"maxUnits"`
	ProgramID ids.ID      `json:"programID"`
	Params    []Parameter `json:"params"`

	Log logging.Logger

	rt runtime.WasmRuntime
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

var _ runtime.ProgramLoader = (*ProgramStore)(nil)

type ProgramStore struct {
	state.Mutable
}

func (s *ProgramStore) GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error) {
	// TODO: take fee out of balance?
	programBytes, exists, err := storage.GetProgram(context.Background(), s, programID)
	if !exists {
		return []byte{}, errors.New("unknown program")
	}
	if err != nil {
		return []byte{}, err
	}

	return programBytes, nil
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
		return false, 1, OutputValueZero, errors.New("no function called")
	}
	if len(t.Params) == 0 {
		return false, 1, OutputValueZero, errors.New("no params passed")
	}

	data, err := ParamsToBytes(t.Params)
	if err != nil {
		return false, 0, nil, err
	}

	s := &ProgramStore {
		Mutable: mu,
	}
	// TODO don't create a new runtime at every execute action
	cfg := runtime.NewConfig()
	// TODO pass the logger
	rt := *runtime.NewRuntime(cfg, t.Log, s)
	callInfo := runtime.CallInfo {
		State: s.Mutable,
		// Actor: ids.ID(actor),
		Actor: ids.Empty,
		ProgramID: t.ProgramID,
		Fuel: t.MaxUnits, // TODO implement me
		FunctionName: t.Function, // TODO don't call _guest functions it is prepended in the sim
		Params: data,
	}
	ret, err := rt.CallProgram(ctx, &callInfo)
	if err != nil {
		return false, 0, ret, err
	}
	// TODO spend fuel here
	return true, 100000, ret, nil

	// // TODO: get cfg from genesis
	// cfg := runtime.NewConfig()
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }

	// ecfg, err := engine.NewConfigBuilder().
	// 	WithDefaultCache(true).
	// 	Build()
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }
	// eng := engine.New(ecfg)

	// // TODO: allow configurable imports?
	// importsBuilder := host.NewImportsBuilder()
	// importsBuilder.Register("state", func() host.Import {
	// 	return pstate.New(logging.NoLog{}, mu)
	// })
	// callContext := &program.Context{
	// 	ProgramID: programID,
	// 	// Actor:            [32]byte(actor[1:]),
	// 	// OriginatingActor: [32]byte(actor[1:])
	// }

	// importsBuilder.Register("program", func() host.Import {
	// 	return importProgram.New(logging.NoLog{}, eng, mu, cfg, callContext)
	// })
	// imports := importsBuilder.Build()

	// t.rt = runtime.New(logging.NoLog{}, eng, imports, cfg)
	// err = t.rt.Initialize(ctx, callContext, programBytes, t.MaxUnits)
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }
	// defer t.rt.Stop()

	// mem, err := t.rt.Memory()
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }
	// params, err := WriteParams(mem, t.Params)
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }

	// resp, err := t.rt.Call(ctx, t.Function, callContext, params[1:]...)
	// if err != nil {
	// 	return false, 1, utils.ErrBytes(err), nil
	// }

	// return true, 1, resp, nil
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
	// return t.rt.Meter().GetBalance()
	return 0, nil
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

// ParamsToBytes is a helper function that writes the given params to memory if non integer.
// Supported types include int, uint64 and string.
func ParamsToBytes(p []CallParam) ([]byte, error) {
	var bytes []byte
	for _, param := range p {
		switch v := param.Value.(type) {
		case []byte:
			bytes = append(bytes, v...)
		case ids.ID:
			bytes = append(bytes, v[:]...)
		case string:
			bytes = append(bytes, []byte(v)...)
		case uint32:
			return nil, errors.New("unsupported type")
			// TODO
			// bytes = append(bytes, v)
		default:
			return nil, errors.New("unsupported type")
			// ptr, err := writeToMem(v, m)
			// if err != nil {
			// 	return nil, err
			// }
			// params = append(params, ptr)
		}
	}

	return bytes, nil
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
