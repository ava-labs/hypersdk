// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	mconsts "github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
)

var _ chain.Action = (*Deploy)(nil)

type Deploy struct {
	ProgramID    runtime.ProgramID `json:"programID"`
	CreationInfo []byte            `json:"creationInfo"`
	address      codec.Address
}

func (*Deploy) GetTypeID() uint8 {
	return mconsts.DeployID
}

func (t *Deploy) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if t.address == codec.EmptyAddress {
		t.address = storage.GetAddressForDeploy(0, t.CreationInfo)
	}
	stateKey, _ := keys.Encode(storage.AccountProgramKey(t.address), 36)
	return state.Keys{
		string(stateKey): state.All,
	}
}

func (*Deploy) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks}
}

func (t *Deploy) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	result, err := (&storage.ProgramStateManager{Mutable: mu}).NewAccountWithProgram(ctx, t.ProgramID, t.CreationInfo)
	return [][]byte{result[:]}, err
}

func (*Deploy) ComputeUnits(chain.Rules) uint64 {
	return 1
}

func (*Deploy) Size() int {
	return codec.AddressLen + consts.Uint64Len
}

func (t *Deploy) Marshal(p *codec.Packer) {
	p.PackBytes(t.ProgramID)
	p.PackBytes(t.CreationInfo)
}

func UnmarshalDeployProgram(p *codec.Packer) (chain.Action, error) {
	var deployProgram Deploy
	p.UnpackBytes(36, true, (*[]byte)(&deployProgram.ProgramID))
	p.UnpackBytes(10*units.MiB, false, &deployProgram.CreationInfo)
	deployProgram.address = storage.GetAddressForDeploy(0, deployProgram.CreationInfo)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &deployProgram, nil
}

func (*Deploy) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
