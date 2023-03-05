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

var _ chain.Action = (*ExportAsset)(nil)

type ExportAsset struct {
	To          crypto.PublicKey `json:"to"`
	Asset       ids.ID           `json:"asset"`
	Value       uint64           `json:"value"`
	Return      bool             `json:"return"`
	Reward      uint64           `json:"reward"`
	Destination ids.ID           `json:"destination"`
}

func (e *ExportAsset) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	var (
		keys  [][]byte
		actor = auth.GetActor(rauth)
	)
	if e.Return {
		keys = [][]byte{
			storage.PrefixAssetKey(e.Asset),
			storage.PrefixBalanceKey(actor, e.Asset),
		}
	} else {
		keys = [][]byte{
			storage.PrefixLoanKey(e.Asset, e.Destination),
			storage.PrefixBalanceKey(actor, e.Asset),
		}
	}

	return keys
}

func (e *ExportAsset) executeReturn(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	actor crypto.PublicKey,
	txID ids.ID,
) (*chain.Result, error) {
	unitsUsed := e.MaxUnits(r)
	exists, metadata, supply, _, isWarp, err := storage.GetAsset(ctx, db, e.Asset)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAssetMissing}, nil
	}
	if !isWarp {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputNotWarpAsset}, nil
	}
	allowedDestination, err := ids.ToID(metadata[consts.IDLen:])
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if allowedDestination != e.Destination {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputWrongDestination}, nil
	}
	newSupply, err := smath.Sub(supply, e.Value)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	newSupply, err = smath.Sub(newSupply, e.Reward)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if newSupply > 0 {
		if err := storage.SetAsset(ctx, db, e.Asset, metadata, newSupply, crypto.EmptyPublicKey, true); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	} else {
		if err := storage.DeleteAsset(ctx, db, e.Asset); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	if err := storage.SubBalance(ctx, db, actor, e.Asset, e.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if e.Reward > 0 {
		if err := storage.SubBalance(ctx, db, actor, e.Asset, e.Reward); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	originalAsset, err := ids.ToID(metadata[:consts.IDLen])
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	wt := &WarpTransfer{
		To:     e.To,
		Asset:  originalAsset,
		Value:  e.Value,
		Return: e.Return,
		Reward: e.Reward,
		TxID:   txID,
	}
	payload, err := wt.Marshal()
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	wm := &warp.UnsignedMessage{
		DestinationChainID: e.Destination,
		// SourceChainID is populated by hypersdk
		Payload: payload,
	}
	return &chain.Result{Success: true, Units: unitsUsed, WarpMessage: wm}, nil
}

func (e *ExportAsset) executeLoan(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	actor crypto.PublicKey,
	txID ids.ID,
) (*chain.Result, error) {
	unitsUsed := e.MaxUnits(r)
	if err := storage.AddLoan(ctx, db, e.Asset, e.Destination, e.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, e.Asset, e.Value); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if e.Reward > 0 {
		if err := storage.AddLoan(ctx, db, e.Asset, e.Destination, e.Reward); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
		if err := storage.SubBalance(ctx, db, actor, e.Asset, e.Reward); err != nil {
			return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
		}
	}
	wt := &WarpTransfer{
		To:     e.To,
		Asset:  e.Asset,
		Value:  e.Value,
		Return: e.Return,
		Reward: e.Reward,
		TxID:   txID,
	}
	payload, err := wt.Marshal()
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	wm := &warp.UnsignedMessage{
		DestinationChainID: e.Destination,
		// SourceChainID is populated by hypersdk
		Payload: payload,
	}
	return &chain.Result{Success: true, Units: unitsUsed, WarpMessage: wm}, nil
}

func (e *ExportAsset) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	txID ids.ID,
	_ *chain.WarpMessage,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := e.MaxUnits(r) // max units == units
	if e.Value == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputValueZero}, nil
	}
	if e.Destination == ids.Empty {
		// This would result in multiplying balance export by whoever imports the
		// transaction.
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputAnycast}, nil
	}
	// TODO: check if destination is ourselves
	if e.Return {
		return e.executeReturn(ctx, r, db, actor, txID)
	}
	return e.executeLoan(ctx, r, db, actor, txID)
}

func (*ExportAsset) MaxUnits(chain.Rules) uint64 {
	return crypto.PublicKeyLen + consts.IDLen*2 + consts.Uint64Len*2 + 1
}

func (e *ExportAsset) Marshal(p *codec.Packer) {
	p.PackPublicKey(e.To)
	p.PackID(e.Asset)
	p.PackUint64(e.Value)
	p.PackBool(e.Return)
	p.PackUint64(e.Reward)
	p.PackID(e.Destination)
}

func UnmarshalExportAsset(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var export ExportAsset
	p.UnpackPublicKey(false, &export.To) // can transfer to blackhole
	p.UnpackID(false, &export.Asset)     // may export native
	export.Value = p.UnpackUint64(true)
	export.Return = p.UnpackBool()
	export.Reward = p.UnpackUint64(false) // reward not required
	p.UnpackID(true, &export.Destination)
	return &export, p.Err()
}

func (*ExportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
