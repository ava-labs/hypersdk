// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/programsvm/storage"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/programsvm/consts"
)

var _ chain.Action = (*CallProgram)(nil)

type CallProgram struct {
	// program is the address of the program to be called
	Program codec.Address `json:"program"`

	// Function is the name of the function to call on the program.
	Function string `json:"function"`

	// CallData are the serialized parameters to be passed to the program.
	CallData []byte `json:"calldata"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// StateKeyList is the list of all touched state keys
	StateKeyList []string `json:"statekeys"`
}

func (*CallProgram) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *CallProgram) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	keys := state.Keys{}
	return keys
}

func (*CallProgram) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *CallProgram) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	return nil, nil
}

func (*CallProgram) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*CallProgram) Size() int {
	return codec.AddressLen + consts.Uint64Len
}

func (t *CallProgram) Marshal(p *codec.Packer) {
	p.PackAddress(t.To)
	p.PackUint64(t.Value)
}

func UnmarshalCallProgram(p *codec.Packer) (chain.Action, error) {
	var callProgram CallProgram
	p.UnpackAddress(&callProgram.To) // we do not verify the typeID is valid
	callProgram.Value = p.UnpackUint64(true)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return &callProgram, nil
}

func (*CallProgram) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
