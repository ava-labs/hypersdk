// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/internal/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	mconsts "github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
)

var _ chain.Action = (*Deploy)(nil)

const MAXCREATIONSIZE = units.MiB

type Deploy struct {
	ProgramID    runtime.ProgramID `json:"programID"`
	CreationInfo []byte            `json:"creationInfo"`
	address      codec.Address
}

func (*Deploy) GetTypeID() uint8 {
	return mconsts.DeployID
}

func (d *Deploy) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if d.address == codec.EmptyAddress {
		d.address = storage.GetAddressForDeploy(0, d.CreationInfo)
	}
	stateKey, _ := keys.Encode(storage.AccountProgramKey(d.address), 36)
	return state.Keys{
		string(stateKey): state.All,
	}
}

func (*Deploy) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks}
}

func (d *Deploy) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	result, err := (&storage.ProgramStateManager{Mutable: mu}).
		NewAccountWithProgram(ctx, d.ProgramID, d.CreationInfo)
	return [][]byte{result[:]}, err
}

func (*Deploy) ComputeUnits(chain.Rules) uint64 {
	return 1
}

func (d *Deploy) Size() int {
	return len(d.CreationInfo) + len(d.ProgramID)
}

func (d *Deploy) Marshal(p *codec.Packer) {
	p.PackBytes(d.ProgramID)
	p.PackBytes(d.CreationInfo)
}

func UnmarshalDeployProgram(p *codec.Packer) (chain.Action, error) {
	var deployProgram Deploy
	p.UnpackBytes(36, true, (*[]byte)(&deployProgram.ProgramID))
	p.UnpackBytes(MAXCREATIONSIZE, false, &deployProgram.CreationInfo)
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
