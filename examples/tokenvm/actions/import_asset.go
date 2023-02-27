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
	// Message is the raw *warp.Message containing a transfer of some assets to
	// a given address.
	Message *warp.Message `json:"warpMessage"`

	// warpTransfer is parsed from the inner *warp.Message
	warpTransfer *WarpTransfer

	// newAsset is the ids.ID of the assetID that the inbound funds are
	// identified by.
	newAsset ids.ID

	// messageLen is used to determine the fee this transaction should pay.
	messageLen int
}

func (i *ImportAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	newAsset := i.warpTransfer.NewAssetID(i.Message.SourceChainID)
	keys := [][]byte{
		storage.PrefixAssetKey(newAsset),
		storage.PrefixBalanceKey(i.warpTransfer.To, newAsset),
		storage.PrefixWarpMessageKey(i.warpTransfer.TxID),
	}
	// If the [warpTransfer] specified a reward, we add the state key to make
	// sure it is paid.
	if i.warpTransfer.Reward > 0 {
		actor := auth.GetActor(rauth)
		keys = append(keys, storage.PrefixBalanceKey(actor, newAsset))
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
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := i.MaxUnits(r) // max units == units
	has, err := storage.HasWarpMessageID(ctx, db, i.warpTransfer.TxID)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if has {
		return &chain.Result{
			Success: false,
			Units:   unitsUsed,
			Output:  OutputDuplicateWarpMessage,
		}, nil
	}
	if i.warpTransfer.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
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
	if err := storage.StoreWarpMessageID(ctx, db, i.warpTransfer.TxID); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (i *ImportAsset) MaxUnits(chain.Rules) uint64 {
	// TODO: add numSigners multipler to fee (should parse from warp.Signature)
	return uint64(i.messageLen)
}

func (i *ImportAsset) Marshal(p *codec.Packer) {
	p.PackBytes(i.Message.Bytes())
}

func UnmarshalImportAsset(p *codec.Packer) (chain.Action, error) {
	var imp ImportAsset
	var msgBytes []byte
	p.UnpackBytes(chain.MaxWarpMessageSize, true, &msgBytes)
	if err := p.Err(); err != nil {
		return nil, err
	}
	imp.messageLen = len(msgBytes)
	msg, err := warp.ParseMessage(msgBytes)
	if err != nil {
		return nil, err
	}
	imp.Message = msg

	// Parse inner message
	warpTransfer, err := UnmarshalWarpTransfer(imp.Message.Payload)
	if err != nil {
		return nil, err
	}
	imp.warpTransfer = warpTransfer
	imp.newAsset = imp.warpTransfer.NewAssetID(imp.Message.SourceChainID)
	return &imp, nil
}

func (*ImportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (i *ImportAsset) WarpMessage() *warp.Message {
	return i.Message
}
