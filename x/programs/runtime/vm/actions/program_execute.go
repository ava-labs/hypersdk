// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/v2/runtime"
	"github.com/ava-labs/hypersdk/x/programs/v2/vm/storage"
)

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	Function string      `json:"programFunction"`
	MaxUnits uint64      `json:"maxUnits"`
	Params   []CallParam `json:"params"`

	Log logging.Logger

	rt runtime.WasmRuntime
}

type ProgramStore struct {
	state.Mutable
}

func (s *ProgramStore) GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error) {
	// TODO: take fee out of balance?
	programBytes, exists, err := storage.GetProgram(ctx, s, programID)
	if err != nil {
		return []byte{}, err
	}
	if !exists {
		return []byte{}, errors.New("unknown program")
	}

	return programBytes, nil
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

	params, err := SerializeParams(t.Params[1:])
	if err != nil {
		return false, 0, []byte{}, err
	}

	cfg := runtime.NewConfig()
	store := &ProgramStore{
		Mutable: mu,
	}
	rt := runtime.NewRuntime(cfg, t.Log, store)
	callInfo := &runtime.CallInfo{
		State:        mu,
		Actor:        ids.Empty,
		Account:      ids.Empty,
		ProgramID:    programID,
		Fuel:         t.MaxUnits,
		FunctionName: t.Function,
		Params:       params,
	}
	output, err = rt.CallProgram(ctx, callInfo)
	if err != nil {
		return false, 0, output, err
	}
	// TODO don't exhaust all fuel here
	return true, 0, output, nil
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
	// TODO implement metering once available
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

func SerializeParams(p []CallParam) ([]byte, error) {
	var bytes []byte
	for _, param := range p {
		switch v := param.Value.(type) {
		case []byte:
			bytes = append(bytes, v...)
		case ids.ID:
			bytes = append(bytes, v[:]...)
		case string:
			bytes = append(bytes, []byte(v)...)
		case uint64:
			bs := make([]byte, 8)
			binary.LittleEndian.PutUint64(bs, v)
			bytes = append(bytes, bs...)
		case uint32:
			bs := make([]byte, 4)
			binary.LittleEndian.PutUint32(bs, v)
			bytes = append(bytes, bs...)
		default:
			return nil, errors.New("unsupported data type")
		}
	}
	return bytes, nil
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
