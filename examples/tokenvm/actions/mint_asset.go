// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	smath "github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*MintAsset)(nil)

type MintAsset struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Asset is the [ActionID] that created the asset.
	Asset codec.LID `json:"asset"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`
}

func (*MintAsset) GetTypeID() uint8 {
	return mintAssetID
}

func (m *MintAsset) StateKeys(codec.Address, codec.LID) state.Keys {
	return state.Keys{
		string(storage.AssetKey(m.Asset)):         state.Read | state.Write,
		string(storage.BalanceKey(m.To, m.Asset)): state.All,
	}
}

func (*MintAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks, storage.BalanceChunks}
}

func (m *MintAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ codec.LID,
) (bool, uint64, [][]byte) {
	if m.Asset == codec.Empty {
		return false, MintAssetComputeUnits, [][]byte{OutputAssetIsNative}
	}
	if m.Value == 0 {
		return false, MintAssetComputeUnits, [][]byte{OutputValueZero}
	}
	exists, symbol, decimals, metadata, supply, owner, err := storage.GetAsset(ctx, mu, m.Asset)
	if err != nil {
		return false, MintAssetComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if !exists {
		return false, MintAssetComputeUnits, [][]byte{OutputAssetMissing}
	}
	if owner != actor {
		return false, MintAssetComputeUnits, [][]byte{OutputWrongOwner}
	}
	newSupply, err := smath.Add64(supply, m.Value)
	if err != nil {
		return false, MintAssetComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if err := storage.SetAsset(ctx, mu, m.Asset, symbol, decimals, metadata, newSupply, actor); err != nil {
		return false, MintAssetComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if err := storage.AddBalance(ctx, mu, m.To, m.Asset, m.Value, true); err != nil {
		return false, MintAssetComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	return true, MintAssetComputeUnits, [][]byte{{}}
}

func (*MintAsset) MaxComputeUnits(chain.Rules) uint64 {
	return MintAssetComputeUnits
}

func (*MintAsset) Size() int {
	return codec.AddressLen + codec.LIDLen + consts.Uint64Len
}

func (m *MintAsset) Marshal(p *codec.Packer) {
	p.PackLID(m.To)
	p.PackLID(m.Asset)
	p.PackUint64(m.Value)
}

func UnmarshalMintAsset(p *codec.Packer) (chain.Action, error) {
	var mint MintAsset
	p.UnpackLID(true, &mint.To)
	p.UnpackLID(true, &mint.Asset) // empty ID is the native asset
	mint.Value = p.UnpackUint64(true)
	return &mint, p.Err()
}

func (*MintAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
