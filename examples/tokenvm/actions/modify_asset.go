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
	"github.com/ava-labs/hypersdk/crypto/ed25519"
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
	Owner ed25519.PublicKey `json:"owner"`

	// Metadata is the new metadata of the [Asset].
	//
	// If you want this to stay the same, you must set it to be the same value.
	Metadata []byte `json:"metadata"`
}

func (*ModifyAsset) GetTypeID() uint8 {
	return modifyAssetID
}

func (m *ModifyAsset) StateKeys(chain.Auth, ids.ID) []string {
	return []string{
		string(storage.AssetKey(m.Asset)),
	}
}

func (*ModifyAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks}
}

func (*ModifyAsset) OutputsWarpMessage() bool {
	return false
}

func (m *ModifyAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	actor := auth.GetActor(rauth)
	if m.Asset == ids.Empty {
		return false, ModifyAssetComputeUnits, OutputAssetIsNative, nil, nil
	}
	if len(m.Metadata) > MaxMetadataSize {
		return false, ModifyAssetComputeUnits, OutputMetadataTooLarge, nil, nil
	}
	exists, _, supply, owner, isWarp, err := storage.GetAsset(ctx, db, m.Asset)
	if err != nil {
		return false, ModifyAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if !exists {
		return false, ModifyAssetComputeUnits, OutputAssetMissing, nil, nil
	}
	if isWarp {
		return false, ModifyAssetComputeUnits, OutputWarpAsset, nil, nil
	}
	if owner != actor {
		return false, ModifyAssetComputeUnits, OutputWrongOwner, nil, nil
	}
	if err := storage.SetAsset(ctx, db, m.Asset, m.Metadata, supply, m.Owner, isWarp); err != nil {
		return false, ModifyAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	return true, ModifyAssetComputeUnits, nil, nil, nil
}

func (*ModifyAsset) MaxComputeUnits(chain.Rules) uint64 {
	return ModifyAssetComputeUnits
}

func (m *ModifyAsset) Size() int {
	return consts.IDLen + ed25519.PublicKeyLen + codec.BytesLen(m.Metadata)
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
