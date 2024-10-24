// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
	"github.com/ava-labs/hypersdk/x/contracts/vm/storage"

	mconsts "github.com/ava-labs/hypersdk/x/contracts/vm/consts"
)

var _ chain.Action = (*Deploy)(nil)

const MAXCREATIONSIZE = units.MiB

type Deploy struct {
	ContractID   runtime.ContractID `json:"contractID"`
	CreationInfo []byte             `json:"creationInfo"`
	address      codec.Address
}

func (*Deploy) GetTypeID() uint8 {
	return mconsts.DeployID
}

func (d *Deploy) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if d.address == codec.EmptyAddress {
		d.address = storage.GetAddressForDeploy(0, d.CreationInfo)
	}
	stateKey, _ := keys.Encode(storage.AccountContractKey(d.address), 36)
	return state.Keys{
		string(stateKey): state.All,
	}
}

func (d *Deploy) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	result, err := (&storage.ContractStateManager{Mutable: mu}).
		NewAccountWithContract(ctx, d.ContractID, d.CreationInfo)
	return &AddressOutput{Address: result}, err
}

func (*Deploy) ComputeUnits(chain.Rules) uint64 {
	return 1
}

func (d *Deploy) Size() int {
	return len(d.CreationInfo) + len(d.ContractID)
}

func (d *Deploy) Marshal(p *codec.Packer) {
	p.PackBytes(d.ContractID)
	p.PackBytes(d.CreationInfo)
}

func UnmarshalDeployContract(p *codec.Packer) (chain.Action, error) {
	var deployContract Deploy
	p.UnpackBytes(36, true, (*[]byte)(&deployContract.ContractID))
	p.UnpackBytes(MAXCREATIONSIZE, false, &deployContract.CreationInfo)
	deployContract.address = storage.GetAddressForDeploy(0, deployContract.CreationInfo)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &deployContract, nil
}

func (*Deploy) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

type AddressOutput struct {
	Address codec.Address `serialize:"true" json:"address"`
}

func (*AddressOutput) GetTypeID() uint8 {
	return mconsts.AddressOutputID
}
