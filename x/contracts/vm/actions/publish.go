// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"crypto/sha256"

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

var _ chain.Action = (*Publish)(nil)

const MAXCONTRACTSIZE = 2 * units.MiB

type Publish struct {
	ContractBytes []byte `json:"contractBytes"`
	id            runtime.ContractID
}

func (*Publish) GetTypeID() uint8 {
	return mconsts.PublishID
}

func (t *Publish) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if t.id == nil {
		hashedID := sha256.Sum256(t.ContractBytes)
		t.id, _ = keys.Encode(storage.ContractsKey(hashedID[:]), len(t.ContractBytes))
	}
	return state.Keys{
		string(t.id): state.Write | state.Allocate,
	}
}

func (t *Publish) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	resultBytes, err := storage.StoreContract(ctx, mu, t.ContractBytes)
	if err != nil {
		return nil, err
	}
	return &Result{Value: resultBytes}, nil
}

func (*Publish) ComputeUnits(chain.Rules) uint64 {
	return 5
}

func (t *Publish) Size() int {
	return 4 + len(t.ContractBytes)
}

func (t *Publish) Marshal(p *codec.Packer) {
	p.PackBytes(t.ContractBytes)
}

func UnmarshalPublishContract(p *codec.Packer) (chain.Action, error) {
	var publishContract Publish
	p.UnpackBytes(MAXCONTRACTSIZE, true, &publishContract.ContractBytes)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &publishContract, nil
}

func (*Publish) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
