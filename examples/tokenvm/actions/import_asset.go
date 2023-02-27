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
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ImportAsset)(nil)

type ImportAsset struct {
	// Message is the raw *warp.Message containing a transfer of some assets to
	// a given address.
	Message *warp.Message `json:"warpMessage"`

	// The below fields are populated by parsing the *warp.Message payload as
	// a transfer action.
	//
	// to is the recipient of the [value].
	to crypto.PublicKey

	// asset is the assetID provided in the *warp.Message. The cannonical asset
	// on this chain will be hash(assetID, sourceChainID).
	asset ids.ID

	// newAsset is the new assetID for this warped asset. It is computed by
	// hashing assetID with the sourceChainID. This ensures that as assets flow
	// through different Subnets that the path they take is reflected in their
	// identity.
	newAsset ids.ID

	// value of asset to transfer to [to].
	value uint64

	// reward is the amount to send to the actor for relaying the message.
	reward uint64

	// txID is the txID of the transaction that created this *warp.Message. We
	// rely on this for replay protection.
	//
	// Because the txID includes the chainID when it is created on the source, it
	// should never collide with another *warp.Message.
	txID ids.ID

	// messageLen is used to determine the fee this transaction should pay.
	messageLen int
}

func (i *ImportAsset) StateKeys(chain.Auth, ids.ID) [][]byte {
	return [][]byte{
		storage.PrefixAssetKey(i.newAsset),
		storage.PrefixBalanceKey(i.to, i.newAsset),
		storage.PrefixWarpMessageKey(i.txID),
	}
}

func (i *ImportAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
) (*chain.Result, error) {
	unitsUsed := i.MaxUnits(r) // max units == units
	has, err := storage.HasWarpMessageID(ctx, db, i.txID)
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
	if i.asset == ids.Empty {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetIsNative}, nil
	}
	if i.value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	exists, metadata, supply, _, err := storage.GetAsset(ctx, db, i.newAsset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		// It is ok if the asset is missing, we'll just create it later
		metadata = []byte(fmt.Sprintf("%s|%s", i.asset, i.Message.SourceChainID))
	}
	newSupply, err := smath.Add64(supply, i.value)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetAsset(ctx, db, i.newAsset, metadata, newSupply, crypto.EmptyPublicKey); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, i.to, i.newAsset, i.value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.StoreWarpMessageID(ctx, db, i.txID); err != nil {
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
	msg, err := warp.ParseMessage(msgBytes)
	if err != nil {
		return nil, err
	}
	imp.Message = msg

	// TODO: define own struct for inner message instead of using transfer
	// Parse inner message
	ip := codec.NewReader(imp.Message.Payload, crypto.PublicKeyLen+consts.IDLen+consts.Uint64Len)
	transferAction, err := UnmarshalTransfer(ip)
	if err != nil {
		return nil, err
	}
	transfer := transferAction.(*Transfer)
	imp.to = transfer.To
	imp.asset = transfer.Asset
	imp.newAsset = utils.ToID(append(imp.asset[:], imp.Message.SourceChainID[:]...))
	imp.value = transfer.Value
	return &imp, nil
}

func (*ImportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (i *ImportAsset) WarpMessage() *warp.Message {
	return i.Message
}
