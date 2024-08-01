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
	"github.com/ava-labs/hypersdk/examples/programsvm/storage"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*PublishProgram)(nil)

type PublishProgram struct {
	ProgramBytes []byte `json:"programBytes"`
	id           ids.ID
}

func (*PublishProgram) GetTypeID() uint8 {
	return 3
}

func (t *PublishProgram) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if t.id == ids.Empty {
		t.id = sha256.Sum256(t.ProgramBytes)
	}
	key, _ := keys.Encode(storage.ProgramsKey(t.id[:]), len(t.ProgramBytes))
	return state.Keys{
		string(key): state.Write | state.Allocate,
	}
}

func (t *PublishProgram) StateKeysMaxChunks() []uint16 {
	return []uint16{uint16(len(t.ProgramBytes))}
}

func (t *PublishProgram) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	result, err := storage.StoreProgram(ctx, mu, t.ProgramBytes)
	return [][]byte{result[:]}, err
}

func (*PublishProgram) ComputeUnits(chain.Rules) uint64 {
	return 5
}

func (t *PublishProgram) Size() int {
	return 4 + len(t.ProgramBytes)
}

func (t *PublishProgram) Marshal(p *codec.Packer) {
	p.PackBytes(t.ProgramBytes)
}

func UnmarshalPublishProgram(p *codec.Packer) (chain.Action, error) {
	var publishProgram PublishProgram
	p.UnpackBytes(10*units.MiB, true, &publishProgram.ProgramBytes)
	if err := p.Err(); err != nil {
		return nil, err
	}

	return &publishProgram, nil
}

func (*PublishProgram) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
