// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	ProgramID ids.ID `json:"programID"`
	Function  string `json:"programFunction"`
	MaxUnits  uint64 `json:"maxUnits"`
	Params    []byte `json:"params"`

	Log logging.Logger
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

func (*ProgramExecute) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
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
) (output [][]byte, err error) {
	if len(t.Function) == 0 {
		return [][]byte{OutputValueZero}, errors.New("no function called")
	}

	cfg := runtime.NewConfig()
	store := &ProgramStore{
		Mutable: mu,
	}
	rt, err := runtime.NewRuntime(cfg, t.Log, store)
	if err != nil {
		return nil, err
	}
	callInfo := &runtime.CallInfo{
		State:        mu,
		Actor:        actor,
		Account:      codec.EmptyAddress,
		ProgramID:    t.ProgramID,
		Fuel:         t.MaxUnits,
		FunctionName: t.Function,
		Params:       t.Params,
	}
	programOutput, err := rt.CallProgram(ctx, callInfo)
	output = [][]byte{programOutput}
	if err != nil {
		return output, err
	}
	// TODO don't exhaust all fuel here
	return output, nil
}

func (*ProgramExecute) ComputeUnits(chain.Rules) uint64 {
	return ProgramExecuteComputeUnits
}

func (*ProgramExecute) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (*ProgramExecute) Marshal(_ *codec.Packer) {
	// TODO
}

func (*ProgramExecute) GetBalance() (uint64, error) {
	// TODO implement metering once available
	// return t.rt.Meter().GetBalance()
	return 0, nil
}

func UnmarshalProgramExecute(_ *codec.Packer) (chain.Action, error) {
	// TODO
	return nil, nil
}

func (*ProgramExecute) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
