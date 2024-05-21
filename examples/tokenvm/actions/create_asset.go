// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*CreateAsset)(nil)

type CreateAsset struct {
	Symbol   []byte `json:"symbol"`
	Decimals uint8  `json:"decimals"`
	Metadata []byte `json:"metadata"`
}

func (*CreateAsset) GetTypeID() uint8 {
	return createAssetID
}

func (*CreateAsset) StateKeys(_ codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.AssetKey(actionID)): state.Allocate | state.Write,
	}
}

func (*CreateAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks}
}

func (c *CreateAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	actionID ids.ID,
) (uint64, [][]byte, error) {
	if len(c.Symbol) == 0 {
		return CreateAssetComputeUnits, nil, ErrOutputSymbolEmpty
	}
	if len(c.Symbol) > MaxSymbolSize {
		return CreateAssetComputeUnits, nil, ErrOutputSymbolTooLarge
	}
	if c.Decimals > MaxDecimals {
		return CreateAssetComputeUnits, nil, ErrOutputDecimalsTooLarge
	}
	if len(c.Metadata) == 0 {
		return CreateAssetComputeUnits, nil, ErrOutputMetadataEmpty
	}
	if len(c.Metadata) > MaxMetadataSize {
		return CreateAssetComputeUnits, nil, ErrOutputMetadataTooLarge
	}
	// It should only be possible to overwrite an existing asset if there is
	// a hash collision.
	if err := storage.SetAsset(ctx, mu, actionID, c.Symbol, c.Decimals, c.Metadata, 0, actor); err != nil {
		return CreateAssetComputeUnits, nil, err
	}
	return CreateAssetComputeUnits, nil, nil
}

func (*CreateAsset) MaxComputeUnits(chain.Rules) uint64 {
	return CreateAssetComputeUnits
}

func (c *CreateAsset) Size() int {
	// TODO: add small bytes (smaller int prefix)
	return codec.BytesLen(c.Symbol) + consts.Uint8Len + codec.BytesLen(c.Metadata)
}

func (c *CreateAsset) Marshal(p *codec.Packer) {
	p.PackBytes(c.Symbol)
	p.PackByte(c.Decimals)
	p.PackBytes(c.Metadata)
}

func UnmarshalCreateAsset(p *codec.Packer) (chain.Action, error) {
	var create CreateAsset
	p.UnpackBytes(MaxSymbolSize, true, &create.Symbol)
	create.Decimals = p.UnpackByte()
	p.UnpackBytes(MaxMetadataSize, true, &create.Metadata)
	return &create, p.Err()
}

func (*CreateAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
