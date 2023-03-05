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
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ImportAsset)(nil)

type ImportAsset struct {
	// warpTransfer is parsed from the inner *warp.Message
	warpTransfer *WarpTransfer

	// warpMessage is the full *warp.Message parsed from [chain.Transaction]
	warpMessage *warp.Message
}

func (i *ImportAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	var (
		keys    [][]byte
		assetID ids.ID
	)
	if i.warpTransfer.Return {
		assetID = i.warpTransfer.Asset
		keys = [][]byte{
			storage.PrefixLoanKey(i.warpTransfer.Asset, i.warpMessage.SourceChainID),
			storage.PrefixBalanceKey(i.warpTransfer.To, i.warpTransfer.Asset),
		}
	} else {
		assetID = i.warpTransfer.NewAssetID(i.warpMessage.SourceChainID)
		keys = [][]byte{
			storage.PrefixAssetKey(assetID),
			storage.PrefixBalanceKey(i.warpTransfer.To, assetID),
		}
	}

	// If the [warpTransfer] specified a reward, we add the state key to make
	// sure it is paid.
	if i.warpTransfer.Reward > 0 {
		actor := auth.GetActor(rauth)
		keys = append(keys, storage.PrefixBalanceKey(actor, assetID))
	}
	return keys
}

func (i *ImportAsset) executeMint(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	actor crypto.PublicKey,
) (*chain.Result, error) {
	asset := i.warpTransfer.NewAssetID(i.warpMessage.SourceChainID)
	unitsUsed := i.MaxUnits(r)
	exists, metadata, supply, _, warp, err := storage.GetAsset(ctx, db, asset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if exists && !warp {
		// Should not be possible
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputConflictingAsset}, nil
	}
	if !exists {
		metadata = make([]byte, consts.IDLen*2)
		copy(metadata, i.warpTransfer.Asset[:])
		copy(metadata[consts.IDLen:], i.warpMessage.SourceChainID[:])
	}
	newSupply, err := smath.Add64(supply, i.warpTransfer.Value)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	newSupply, err = smath.Add64(newSupply, i.warpTransfer.Reward)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetAsset(ctx, db, asset, metadata, newSupply, crypto.EmptyPublicKey, true); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, i.warpTransfer.To, asset, i.warpTransfer.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.AddBalance(ctx, db, actor, asset, i.warpTransfer.Reward); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *ImportAsset) executeReturn(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	actor crypto.PublicKey,
) (*chain.Result, error) {
	unitsUsed := i.MaxUnits(r)
	if err := storage.SubLoan(
		ctx, db, i.warpTransfer.Asset,
		i.warpMessage.SourceChainID, i.warpTransfer.Value,
	); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(
		ctx, db, i.warpTransfer.To,
		i.warpTransfer.Asset, i.warpTransfer.Value,
	); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.SubLoan(
			ctx, db, i.warpTransfer.Asset,
			i.warpMessage.SourceChainID, i.warpTransfer.Reward,
		); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if err := storage.AddBalance(
			ctx, db, actor,
			i.warpTransfer.Asset, i.warpTransfer.Reward,
		); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *ImportAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	wm *chain.WarpMessage,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := i.MaxUnits(r) // max units == units
	if wm.VerifyErr != nil {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  utils.ErrBytes(wm.VerifyErr),
		}, nil
	}
	if i.warpTransfer.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	if i.warpTransfer.Return {
		return i.executeReturn(ctx, r, db, actor)
	}
	return i.executeMint(ctx, r, db, actor)
}

func (i *ImportAsset) MaxUnits(chain.Rules) uint64 {
	// TODO: ensure we protect this from warp
	return uint64(len(i.warpMessage.Payload))
}

// All we encode that is action specific for now is the type byte from the
// registry.
func (*ImportAsset) Marshal(*codec.Packer) {}

func UnmarshalImportAsset(_ *codec.Packer, wm *warp.Message) (chain.Action, error) {
	var (
		imp ImportAsset
		err error
	)
	imp.warpMessage = wm
	imp.warpTransfer, err = UnmarshalWarpTransfer(imp.warpMessage.Payload)
	if err != nil {
		return nil, err
	}
	return &imp, nil
}

func (*ImportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
