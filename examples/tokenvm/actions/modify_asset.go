// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ModifyAsset)(nil)

type ModifyAsset struct {
	// Asset is the [TxID] that created the asset.
	Asset ids.ID

	// Owner will be the new owner of the [Asset].
	//
	// If you want to retain ownership, set this to the signer. If you want to
	// revoke ownership, set this to another key or the empty public key.
	Owner crypto.PublicKey `json:"owner"`

	// Metadata is the new metadata of the [Asset].
	//
	// If you want this to stay the same, you must set it to be the same value.
	Metadata []byte `json:"metadata"`
}

func (m *ModifyAsset) StateKeys(chain.Auth, ids.ID) [][]byte {
	return [][]byte{storage.PrefixAssetKey(m.Asset)}
}

func (m *ModifyAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := m.MaxUnits(r) // max units == units
	if m.Asset == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetIsNative}, nil
	}
	if len(m.Metadata) > MaxMetadataSize {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputMetadataTooLarge}, nil
	}
	exists, _, supply, owner, isWarp, err := storage.GetAsset(ctx, db, m.Asset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetMissing}, nil
	}
	if isWarp {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputWarpAsset}, nil
	}
	if owner != actor {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputWrongOwner,
		}, nil
	}
	if err := storage.SetAsset(ctx, db, m.Asset, m.Metadata, supply, m.Owner, isWarp); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (m *ModifyAsset) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return consts.IDLen + crypto.PublicKeyLen + uint64(len(m.Metadata))
}

func (m *ModifyAsset) Marshal(p *codec.Packer) {
	p.PackID(m.Asset)
	p.PackPublicKey(m.Owner)
	p.PackBytes(m.Metadata)
}

func UnmarshalModifyAsset(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var modify ModifyAsset
	p.UnpackID(true, &modify.Asset)         // empty ID is the native asset
	p.UnpackPublicKey(false, &modify.Owner) // empty revokes ownership
	p.UnpackBytes(MaxMetadataSize, false, &modify.Metadata)
	return &modify, p.Err()
}

func (*ModifyAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
