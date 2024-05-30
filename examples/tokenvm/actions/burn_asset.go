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

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ chain.Action = (*BurnAsset)(nil)

type BurnAsset struct {
	// Asset is the [ActionID] that created the asset.
	Asset ids.ID `json:"asset"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`
}

func (*BurnAsset) GetTypeID() uint8 {
	return burnAssetID
}

func (b *BurnAsset) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.AssetKey(b.Asset)):          state.Read | state.Write,
		string(storage.BalanceKey(actor, b.Asset)): state.Read | state.Write,
	}
}

func (*BurnAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks, storage.BalanceChunks}
}

func (b *BurnAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	if b.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if err := storage.SubBalance(ctx, mu, actor, b.Asset, b.Value); err != nil {
		return nil, err
	}
	exists, symbol, decimals, metadata, supply, owner, err := storage.GetAsset(ctx, mu, b.Asset)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrOutputAssetMissing
	}
	newSupply, err := smath.Sub(supply, b.Value)
	if err != nil {
		return nil, err
	}
	if err := storage.SetAsset(ctx, mu, b.Asset, symbol, decimals, metadata, newSupply, owner); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*BurnAsset) ComputeUnits(chain.Rules) uint64 {
	return BurnComputeUnits
}

func (*BurnAsset) Size() int {
	return ids.IDLen + consts.Uint64Len
}

func (b *BurnAsset) Marshal(p *codec.Packer) {
	p.PackID(b.Asset)
	p.PackUint64(b.Value)
}

func UnmarshalBurnAsset(p *codec.Packer) (chain.Action, error) {
	var burn BurnAsset
	p.UnpackID(false, &burn.Asset) // can burn native asset
	burn.Value = p.UnpackUint64(true)
	return &burn, p.Err()
}

func (*BurnAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
