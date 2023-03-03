// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"fmt"

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
	// warpTransfer is parsed from the inner *warp.Message
	warpTransfer *WarpTransfer

	// warpMessage is the full *warp.Message parsed from [chain.Transaction]
	warpMessage *warp.Message
}

func (i *ImportAsset) assetID() ids.ID {
	if i.warpTransfer.Return {
		return i.warpTransfer.Asset
	}
	return i.warpTransfer.NewAssetID(i.warpMessage.SourceChainID)
}

func (i *ImportAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	var (
		keys    [][]byte
		assetID ids.ID
	)
	if i.warpTransfer.Return {
		keys = [][]byte{
			// TODO: add loan item
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
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(wm.VerifyErr)}, nil
	}
	if i.warpTransfer.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	// TODO: must add special handling if bringing funds back to their home chain
	exists, metadata, supply, _, err := storage.GetAsset(ctx, db, i.newAsset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		// It is ok if the asset is missing, we'll just create it later
		metadata = []byte(fmt.Sprintf("%s|%s", i.warpTransfer.Asset, i.Message.SourceChainID))
	}
	newSupply, err := smath.Add64(supply, i.warpTransfer.Value)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	newSupply, err = smath.Add64(supply, i.warpTransfer.Reward)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetAsset(ctx, db, i.newAsset, metadata, newSupply, crypto.EmptyPublicKey); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, i.warpTransfer.To, i.newAsset, i.warpTransfer.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if i.warpTransfer.Reward > 0 {
		if err := storage.AddBalance(ctx, db, actor, i.newAsset, i.warpTransfer.Reward); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *ImportAsset) MaxUnits(chain.Rules) uint64 {
	return uint64(len(i.warpMessage.Payload))
}

func (i *ImportAsset) Marshal(p *codec.Packer) {}

func UnmarshalImportAsset(p *codec.Packer, wm *warp.Message) (chain.Action, error) {
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
