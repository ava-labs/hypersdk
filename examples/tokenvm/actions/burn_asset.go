// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*BurnAsset)(nil)

type BurnAsset struct {
	// Asset is the [TxID] that created the asset.
	Asset ids.ID `json:"asset"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`
}

func (b *BurnAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{
		storage.PrefixAssetKey(b.Asset),
		storage.PrefixBalanceKey(actor, b.Asset),
	}
}

func (b *BurnAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := b.MaxUnits(r) // max units == units
	if b.Asset == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetIsNative}, nil
	}
	if b.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	metadata, supply, owner, err := storage.GetAsset(ctx, db, b.Asset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if owner != actor {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputWrongOwner,
		}, nil
	}
	newSupply, err := smath.Sub(supply, b.Value)
	if err != nil {
		// This should never fail
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetAsset(ctx, db, b.Asset, metadata, newSupply, actor); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, b.Asset, b.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*BurnAsset) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return consts.IDLen + consts.Uint64Len
}

func (b *BurnAsset) Marshal(p *codec.Packer) {
	p.PackID(b.Asset)
	p.PackUint64(b.Value)
}

func UnmarshalBurnAsset(p *codec.Packer) (chain.Action, error) {
	var burn BurnAsset
	p.UnpackID(true, &burn.Asset) // empty ID is the native asset
	burn.Value = p.UnpackUint64(true)
	return &burn, p.Err()
}

func (*BurnAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
