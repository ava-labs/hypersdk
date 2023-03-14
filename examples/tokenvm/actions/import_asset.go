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
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
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

func (i *ImportAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	var (
		keys    [][]byte
		assetID ids.ID
		actor   = auth.GetActor(rauth)
	)
	if i.warpTransfer.Return {
		assetID = i.warpTransfer.Asset
		keys = [][]byte{
			storage.PrefixLoanKey(i.warpTransfer.Asset, i.warpMessage.SourceChainID),
			storage.PrefixBalanceKey(i.warpTransfer.To, i.warpTransfer.Asset),
		}
	} else {
		assetID = ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
		keys = [][]byte{
			storage.PrefixAssetKey(assetID),
			storage.PrefixBalanceKey(i.warpTransfer.To, assetID),
		}
	}

	// If the [warpTransfer] specified a reward, we add the state key to make
	// sure it is paid.
	if i.warpTransfer.Reward > 0 {
		keys = append(keys, storage.PrefixBalanceKey(actor, assetID))
	}

	// If the [warpTransfer] requests a swap, we add the state keys to transfer
	// the required balances.
	if i.Fill && i.warpTransfer.SwapIn > 0 {
		keys = append(keys, storage.PrefixBalanceKey(actor, i.warpTransfer.AssetOut))
		keys = append(keys, storage.PrefixBalanceKey(actor, assetID))
		keys = append(keys, storage.PrefixBalanceKey(i.warpTransfer.To, i.warpTransfer.AssetOut))
	}
	return keys
}

func (i *ImportAsset) executeMint(
	ctx context.Context,
	db chain.Database,
	actor crypto.PublicKey,
) []byte {
	asset := ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
	exists, metadata, supply, _, warp, err := storage.GetAsset(ctx, db, asset)
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
	if err := storage.SetAsset(ctx, db, asset, metadata, newSupply, crypto.EmptyPublicKey, true); err != nil {
		return utils.ErrBytes(err)
	}
	if err := storage.AddBalance(ctx, db, i.warpTransfer.To, asset, i.warpTransfer.Value); err != nil {
		return utils.ErrBytes(err)
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.AddBalance(ctx, db, actor, asset, i.warpTransfer.Reward); err != nil {
			return utils.ErrBytes(err)
		}
	}
	return nil
}

func (i *ImportAsset) executeReturn(
	ctx context.Context,
	db chain.Database,
	actor crypto.PublicKey,
) []byte {
	if err := storage.SubLoan(
		ctx, db, i.warpTransfer.Asset,
		i.warpMessage.SourceChainID, i.warpTransfer.Value,
	); err != nil {
		return utils.ErrBytes(err)
	}
	if err := storage.AddBalance(
		ctx, db, i.warpTransfer.To,
		i.warpTransfer.Asset, i.warpTransfer.Value,
	); err != nil {
		return utils.ErrBytes(err)
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.SubLoan(
			ctx, db, i.warpTransfer.Asset,
			i.warpMessage.SourceChainID, i.warpTransfer.Reward,
		); err != nil {
			return utils.ErrBytes(err)
		}
		if err := storage.AddBalance(
			ctx, db, actor,
			i.warpTransfer.Asset, i.warpTransfer.Reward,
		); err != nil {
			return utils.ErrBytes(err)
		}
	}
	return nil
}

func (i *ImportAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	t int64,
	rauth chain.Auth,
	_ ids.ID,
	warpVerified bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := i.MaxUnits(r) // max units == units
	if !warpVerified {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputWarpVerificationFailed,
		}, nil
	}
	if i.warpTransfer.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	var output []byte
	if i.warpTransfer.Return {
		output = i.executeReturn(ctx, db, actor)
	} else {
		output = i.executeMint(ctx, db, actor)
	}
	if len(output) > 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: output}, nil
	}
	if i.warpTransfer.SwapIn == 0 {
		// We are ensured that [i.Fill] is false here because of logic in unmarshal
		return &chain.Result{Success: true, Units: unitsUsed}, nil
	}
	if !i.Fill {
		if i.warpTransfer.SwapExpiry > t {
			return &chain.Result{Success: false, Units: unitsUsed, Output: OutputMustFill}, nil
		}
		return &chain.Result{Success: true, Units: unitsUsed}, nil
	}
	// TODO: charge more if swap is performed
	var assetIn ids.ID
	if i.warpTransfer.Return {
		assetIn = i.warpTransfer.Asset
	} else {
		assetIn = ImportedAssetID(i.warpTransfer.Asset, i.warpMessage.SourceChainID)
	}
	if err := storage.SubBalance(ctx, db, i.warpTransfer.To, assetIn, i.warpTransfer.SwapIn); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, actor, assetIn, i.warpTransfer.SwapIn); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, i.warpTransfer.AssetOut, i.warpTransfer.SwapOut); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, i.warpTransfer.To, i.warpTransfer.AssetOut, i.warpTransfer.SwapOut); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *ImportAsset) MaxUnits(chain.Rules) uint64 {
	return uint64(len(i.warpMessage.Payload)) + 1
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
