// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	pconsts "github.com/ava-labs/hypersdk/examples/programsvm/consts"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/programsvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*DeployProgram)(nil)

type DeployProgram struct {
	ProgramID    ids.ID `json:"programID"`
	CreationInfo []byte `json:"creationInfo"`
	address      codec.Address
}

func (*DeployProgram) GetTypeID() uint8 {
	return pconsts.DeployProgramID
}

func (t *DeployProgram) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if t.address == codec.EmptyAddress {
		t.InitializeAddress()
	}
	keys := state.Keys{
		string(storage.AccountProgramKey(t.address)): state.All,
	}
	return keys
}

func (*DeployProgram) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *DeployProgram) Execute(
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

func (*DeployProgram) ComputeUnits(chain.Rules) uint64 {
	return PublishComputeUnits
}

func (*DeployProgram) Size() int {
	return codec.AddressLen + consts.Uint64Len
}

func (t *DeployProgram) Marshal(p *codec.Packer) {
	p.PackID(t.ProgramID)
	p.PackBytes(t.CreationInfo)
}

func UnmarshalDeployProgram(p *codec.Packer) (chain.Action, error) {
	var deployProgram DeployProgram
	p.UnpackID(true, &deployProgram.ProgramID)
	p.UnpackBytes(10*units.MiB, true, &deployProgram.CreationInfo)
	deployProgram.InitializeAddress()
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &deployProgram, nil
}

func (*DeployProgram) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (t *DeployProgram) InitializeAddress() {
	t.address = storage.GetAddressForDeploy(0, t.CreationInfo)
}
