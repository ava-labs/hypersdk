// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/programsvm/storage"
	"github.com/ava-labs/hypersdk/state"

	pconsts "github.com/ava-labs/hypersdk/examples/programsvm/consts"
)

var _ chain.Action = (*CallProgram)(nil)

type CallProgram struct {
	// program is the address of the program to be called
	Program codec.Address `json:"program"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// Function is the name of the function to call on the program.
	Function string `json:"function"`

	// CallData are the serialized parameters to be passed to the program.
	CallData []byte `json:"calldata"`
}

func (*CallProgram) GetTypeID() uint8 {
	return pconsts.CallProgramID
}

func (t *CallProgram) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	recorder := &storage.Recorder{State: pconsts.StateKeysDB}
	pconsts.ProgramRuntime.CallProgram(context.Background(), &runtime.CallInfo{
		Program:      t.Program,
		Actor:        actor,
		State:        &storage.ProgramStateManager{Mutable: recorder},
		FunctionName: t.Function,
		Params:       t.CallData,
	})
	return recorder.GetStateKeys()
}

func (*CallProgram) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *CallProgram) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	result, err := pconsts.ProgramRuntime.CallProgram(ctx, &runtime.CallInfo{
		Program:      t.Program,
		Actor:        actor,
		State:        &storage.ProgramStateManager{Mutable: mu},
		FunctionName: t.Function,
		Params:       t.CallData,
		Timestamp:    uint64(timestamp),
	})
	return [][]byte{result}, err
}

func (*CallProgram) ComputeUnits(chain.Rules) uint64 {
	return CallComputeUnits
}

func (*CallProgram) Size() int {
	return codec.AddressLen + consts.Uint64Len
}

func (t *CallProgram) Marshal(p *codec.Packer) {
	p.PackAddress(t.Program)
	p.PackUint64(t.Value)
	p.PackString(t.Function)
	p.PackBytes(t.CallData)
}

func UnmarshalCallProgram(p *codec.Packer) (chain.Action, error) {
	var callProgram CallProgram
	p.UnpackAddress(&callProgram.Program) // we do not verify the typeID is valid
	callProgram.Value = p.UnpackUint64(true)
	callProgram.Function = p.UnpackString(true)
	p.UnpackBytes(units.MiB, true, &callProgram.CallData)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &callProgram, nil
}

func (*CallProgram) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
