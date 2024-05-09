// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
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

func (*CreateAsset) StateKeys(_ codec.Address, actionID codec.LID) state.Keys {
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
	actionID codec.LID,
) (bool, uint64, [][]byte) {
	if len(c.Symbol) == 0 {
		return false, CreateAssetComputeUnits, [][]byte{OutputSymbolEmpty}
	}
	if len(c.Symbol) > MaxSymbolSize {
		return false, CreateAssetComputeUnits, [][]byte{OutputSymbolTooLarge}
	}
	if c.Decimals > MaxDecimals {
		return false, CreateAssetComputeUnits, [][]byte{OutputDecimalsTooLarge}
	}
	if len(c.Metadata) == 0 {
		return false, CreateAssetComputeUnits, [][]byte{OutputMetadataEmpty}
	}
	if len(c.Metadata) > MaxMetadataSize {
		return false, CreateAssetComputeUnits, [][]byte{OutputMetadataTooLarge}
	}
	// It should only be possible to overwrite an existing asset if there is
	// a hash collision.
	if err := storage.SetAsset(ctx, mu, actionID, c.Symbol, c.Decimals, c.Metadata, 0, actor); err != nil {
		return false, CreateAssetComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	return true, CreateAssetComputeUnits, [][]byte{{}}
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
