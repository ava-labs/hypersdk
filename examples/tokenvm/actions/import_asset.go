// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ImportAsset)(nil)

type ImportAsset struct {
	// Fill indicates if the actor wishes to fill the order request in the warp
	// message. This must be true if the warp message is in a block with
	// a timestamp < [SwapExpiry].
	Fill bool `json:"fill"`

	// warpTransfer is parsed from the inner *warp.Message
	warpTransfer *WarpTransfer

	// warpMessage is the full *warp.Message parsed from [chain.Transaction]
	warpMessage *warp.Message
}

func (*ImportAsset) GetTypeID() uint8 {
	return importAssetID
}

func (i *ImportAsset) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	var (
		keys    []string
		assetID ids.ID
		actor   = auth.GetActor(rauth)
	)
	if i.warpTransfer.Return {
		assetID = i.warpTransfer.Asset
		keys = []string{
			string(storage.LoanKey(i.warpTransfer.Asset, i.warpMessage.SourceChainID)),
			string(storage.BalanceKey(i.warpTransfer.To, i.warpTransfer.Asset)),
		}
	} else {
		assetID = ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
		keys = []string{
			string(storage.AssetKey(assetID)),
			string(storage.BalanceKey(i.warpTransfer.To, assetID)),
		}
	}

	// If the [warpTransfer] specified a reward, we add the state key to make
	// sure it is paid.
	if i.warpTransfer.Reward > 0 {
		keys = append(keys, string(storage.BalanceKey(actor, assetID)))
	}

	// If the [warpTransfer] requests a swap, we add the state keys to transfer
	// the required balances.
	if i.Fill && i.warpTransfer.SwapIn > 0 {
		keys = append(keys, string(storage.BalanceKey(actor, i.warpTransfer.AssetOut)))
		keys = append(keys, string(storage.BalanceKey(actor, assetID)))
		keys = append(keys, string(storage.BalanceKey(i.warpTransfer.To, i.warpTransfer.AssetOut)))
	}
	return keys
}

func (i *ImportAsset) StateKeysMaxChunks() []uint16 {
	// Can't use [warpTransfer] because it may not be populated yet
	chunks := []uint16{}
	chunks = append(chunks, storage.LoanChunks)
	chunks = append(chunks, storage.AssetChunks)
	chunks = append(chunks, storage.BalanceChunks)

	// If the [warpTransfer] specified a reward, we add the state key to make
	// sure it is paid.
	chunks = append(chunks, storage.BalanceChunks)

	// If the [warpTransfer] requests a swap, we add the state keys to transfer
	// the required balances.
	if i.Fill {
		chunks = append(chunks, storage.BalanceChunks)
		chunks = append(chunks, storage.BalanceChunks)
		chunks = append(chunks, storage.BalanceChunks)
	}
	return chunks
}

func (*ImportAsset) OutputsWarpMessage() bool {
	return false
}

func (i *ImportAsset) executeMint(
	ctx context.Context,
	mu state.Mutable,
	actor ed25519.PublicKey,
) []byte {
	asset := ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
	exists, metadata, supply, _, warp, err := storage.GetAsset(ctx, mu, asset)
	if err != nil {
		return utils.ErrBytes(err)
	}
	if exists && !warp {
		// Should not be possible
		return OutputConflictingAsset
	}
	if !exists {
		metadata = ImportedAssetMetadata(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
	}
	newSupply, err := smath.Add64(supply, i.warpTransfer.Value)
	if err != nil {
		return utils.ErrBytes(err)
	}
	newSupply, err = smath.Add64(newSupply, i.warpTransfer.Reward)
	if err != nil {
		return utils.ErrBytes(err)
	}
	if err := storage.SetAsset(ctx, mu, asset, metadata, newSupply, ed25519.EmptyPublicKey, true); err != nil {
		return utils.ErrBytes(err)
	}
	if err := storage.AddBalance(ctx, mu, i.warpTransfer.To, asset, i.warpTransfer.Value, true); err != nil {
		return utils.ErrBytes(err)
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.AddBalance(ctx, mu, actor, asset, i.warpTransfer.Reward, true); err != nil {
			return utils.ErrBytes(err)
		}
	}
	return nil
}

func (i *ImportAsset) executeReturn(
	ctx context.Context,
	mu state.Mutable,
	actor ed25519.PublicKey,
) []byte {
	if err := storage.SubLoan(
		ctx, mu, i.warpTransfer.Asset,
		i.warpMessage.SourceChainID, i.warpTransfer.Value,
	); err != nil {
		return utils.ErrBytes(err)
	}
	if err := storage.AddBalance(
		ctx, mu, i.warpTransfer.To,
		i.warpTransfer.Asset, i.warpTransfer.Value,
		true,
	); err != nil {
		return utils.ErrBytes(err)
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.SubLoan(
			ctx, mu, i.warpTransfer.Asset,
			i.warpMessage.SourceChainID, i.warpTransfer.Reward,
		); err != nil {
			return utils.ErrBytes(err)
		}
		if err := storage.AddBalance(
			ctx, mu, actor,
			i.warpTransfer.Asset, i.warpTransfer.Reward,
			true,
		); err != nil {
			return utils.ErrBytes(err)
		}
	}
	return nil
}

func (i *ImportAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	mu state.Mutable,
	t int64,
	rauth chain.Auth,
	_ ids.ID,
	warpVerified bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	actor := auth.GetActor(rauth)
	if !warpVerified {
		return false, ImportAssetComputeUnits, OutputWarpVerificationFailed, nil, nil
	}
	if i.warpTransfer.DestinationChainID != r.ChainID() {
		return false, ImportAssetComputeUnits, OutputInvalidDestination, nil, nil
	}
	if i.warpTransfer.Value == 0 {
		return false, ImportAssetComputeUnits, OutputValueZero, nil, nil
	}
	var output []byte
	if i.warpTransfer.Return {
		output = i.executeReturn(ctx, mu, actor)
	} else {
		output = i.executeMint(ctx, mu, actor)
	}
	if len(output) > 0 {
		return false, ImportAssetComputeUnits, output, nil, nil
	}
	if i.warpTransfer.SwapIn == 0 {
		// We are ensured that [i.Fill] is false here because of logic in unmarshal
		return true, ImportAssetComputeUnits, nil, nil, nil
	}
	if !i.Fill {
		if i.warpTransfer.SwapExpiry > t {
			return false, ImportAssetComputeUnits, OutputMustFill, nil, nil
		}
		return true, ImportAssetComputeUnits, nil, nil, nil
	}
	// TODO: charge more if swap is performed
	var assetIn ids.ID
	if i.warpTransfer.Return {
		assetIn = i.warpTransfer.Asset
	} else {
		assetIn = ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
	}
	if err := storage.SubBalance(ctx, mu, i.warpTransfer.To, assetIn, i.warpTransfer.SwapIn); err != nil {
		return false, ImportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, mu, actor, assetIn, i.warpTransfer.SwapIn, true); err != nil {
		return false, ImportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.SubBalance(ctx, mu, actor, i.warpTransfer.AssetOut, i.warpTransfer.SwapOut); err != nil {
		return false, ImportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, mu, i.warpTransfer.To, i.warpTransfer.AssetOut, i.warpTransfer.SwapOut, true); err != nil {
		return false, ImportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	return true, ImportAssetComputeUnits, nil, nil, nil
}

func (*ImportAsset) MaxComputeUnits(chain.Rules) uint64 {
	return ImportAssetComputeUnits
}

func (*ImportAsset) Size() int {
	return consts.BoolLen
}

// All we encode that is action specific for now is the type byte from the
// registry.
func (i *ImportAsset) Marshal(p *codec.Packer) {
	p.PackBool(i.Fill)
}

func UnmarshalImportAsset(p *codec.Packer, wm *warp.Message) (chain.Action, error) {
	var (
		imp ImportAsset
		err error
	)
	imp.Fill = p.UnpackBool()
	if err := p.Err(); err != nil {
		return nil, err
	}
	imp.warpMessage = wm
	imp.warpTransfer, err = UnmarshalWarpTransfer(imp.warpMessage.Payload)
	if err != nil {
		return nil, err
	}
	// Ensure we can fill the swap if it exists
	if imp.Fill && imp.warpTransfer.SwapIn == 0 {
		return nil, ErrNoSwapToFill
	}
	return &imp, nil
}

func (*ImportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
