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
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/internal/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	mconsts "github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
)

var _ chain.Action = (*Publish)(nil)

const MAXCONTRACTSIZE = 2 * units.MiB

type Publish struct {
	ContractBytes []byte `json:"contractBytes"`
	id            runtime.ProgramID
}

func (*Publish) GetTypeID() uint8 {
	return mconsts.PublishID
}

func (t *Publish) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if t.id == nil {
		hashedID := sha256.Sum256(t.ContractBytes)
		t.id, _ = keys.Encode(storage.ProgramsKey(hashedID[:]), len(t.ContractBytes))
	}
	return state.Keys{
		string(t.id): state.Write | state.Allocate,
	}
}

func (t *Publish) StateKeysMaxChunks() []uint16 {
	if chunks, ok := keys.NumChunks(t.ContractBytes); ok {
		return []uint16{chunks}
	}
	return []uint16{consts.MaxUint16}
}

func (t *Publish) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	result, err := storage.StoreProgram(ctx, mu, t.ContractBytes)
	return [][]byte{result}, err
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

func UnmarshalPublishProgram(p *codec.Packer) (chain.Action, error) {
	var publishProgram Publish
	p.UnpackBytes(MAXCONTRACTSIZE, true, &publishProgram.ContractBytes)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &publishProgram, nil
}

func (*Publish) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
