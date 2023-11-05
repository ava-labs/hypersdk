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
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*ExportAsset)(nil)

type ExportAsset struct {
	To          codec.Address `json:"to"`
	Asset       ids.ID        `json:"asset"`
	Value       uint64        `json:"value"`
	Return      bool          `json:"return"`
	Reward      uint64        `json:"reward"`
	SwapIn      uint64        `json:"swapIn"`
	AssetOut    ids.ID        `json:"assetOut"`
	SwapOut     uint64        `json:"swapOut"`
	SwapExpiry  int64         `json:"swapExpiry"`
	Destination ids.ID        `json:"destination"`
}

func (*ExportAsset) GetTypeID() uint8 {
	return exportAssetID
}

func (e *ExportAsset) StateKeys(auth chain.Auth, _ ids.ID) []string {
	if e.Return {
		return []string{
			string(storage.AssetKey(e.Asset)),
			string(storage.BalanceKey(auth.Actor(), e.Asset)),
		}
	}
	return []string{
		string(storage.AssetKey(e.Asset)),
		string(storage.LoanKey(e.Asset, e.Destination)),
		string(storage.BalanceKey(auth.Actor(), e.Asset)),
	}
}

func (e *ExportAsset) StateKeysMaxChunks() []uint16 {
	if e.Return {
		return []uint16{storage.AssetChunks, storage.BalanceChunks}
	}
	return []uint16{storage.AssetChunks, storage.LoanChunks, storage.BalanceChunks}
}

func (*ExportAsset) OutputsWarpMessage() bool {
	return true
}

func (e *ExportAsset) executeReturn(
	ctx context.Context,
	mu state.Mutable,
	actor codec.Address,
	txID ids.ID,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	exists, symbol, decimals, metadata, supply, _, isWarp, err := storage.GetAsset(ctx, mu, e.Asset)
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if !exists {
		return false, ExportAssetComputeUnits, OutputAssetMissing, nil, nil
	}
	if !isWarp {
		return false, ExportAssetComputeUnits, OutputNotWarpAsset, nil, nil
	}
	allowedDestination, err := ids.ToID(metadata[consts.IDLen:])
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if allowedDestination != e.Destination {
		return false, ExportAssetComputeUnits, OutputWrongDestination, nil, nil
	}
	newSupply, err := smath.Sub(supply, e.Value)
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	newSupply, err = smath.Sub(newSupply, e.Reward)
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if newSupply > 0 {
		if err := storage.SetAsset(ctx, mu, e.Asset, symbol, decimals, metadata, newSupply, codec.EmptyAddress, true); err != nil {
			return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
		}
	} else {
		if err := storage.DeleteAsset(ctx, mu, e.Asset); err != nil {
			return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
		}
	}
	if err := storage.SubBalance(ctx, mu, actor, e.Asset, e.Value); err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if e.Reward > 0 {
		if err := storage.SubBalance(ctx, mu, actor, e.Asset, e.Reward); err != nil {
			return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
		}
	}
	originalAsset, err := ids.ToID(metadata[:consts.IDLen])
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	wt := &WarpTransfer{
		To:                 e.To,
		Symbol:             symbol,
		Decimals:           decimals,
		Asset:              originalAsset,
		Value:              e.Value,
		Return:             e.Return,
		Reward:             e.Reward,
		SwapIn:             e.SwapIn,
		AssetOut:           e.AssetOut,
		SwapOut:            e.SwapOut,
		SwapExpiry:         e.SwapExpiry,
		TxID:               txID,
		DestinationChainID: e.Destination,
	}
	payload, err := wt.Marshal()
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	wm := &warp.UnsignedMessage{
		// NetworkID + SourceChainID is populated by hypersdk
		Payload: payload,
	}
	return true, ExportAssetComputeUnits, nil, wm, nil
}

func (e *ExportAsset) executeLoan(
	ctx context.Context,
	mu state.Mutable,
	actor codec.Address,
	txID ids.ID,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	exists, symbol, decimals, _, _, _, isWarp, err := storage.GetAsset(ctx, mu, e.Asset)
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if !exists {
		return false, ExportAssetComputeUnits, OutputAssetMissing, nil, nil
	}
	if isWarp {
		// Cannot export an asset if it was warped in and not returning
		return false, ExportAssetComputeUnits, OutputWarpAsset, nil, nil
	}
	if err := storage.AddLoan(ctx, mu, e.Asset, e.Destination, e.Value); err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.SubBalance(ctx, mu, actor, e.Asset, e.Value); err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if e.Reward > 0 {
		if err := storage.AddLoan(ctx, mu, e.Asset, e.Destination, e.Reward); err != nil {
			return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
		}
		if err := storage.SubBalance(ctx, mu, actor, e.Asset, e.Reward); err != nil {
			return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
		}
	}
	wt := &WarpTransfer{
		To:                 e.To,
		Symbol:             symbol,
		Decimals:           decimals,
		Asset:              e.Asset,
		Value:              e.Value,
		Return:             e.Return,
		Reward:             e.Reward,
		SwapIn:             e.SwapIn,
		AssetOut:           e.AssetOut,
		SwapOut:            e.SwapOut,
		SwapExpiry:         e.SwapExpiry,
		TxID:               txID,
		DestinationChainID: e.Destination,
	}
	payload, err := wt.Marshal()
	if err != nil {
		return false, ExportAssetComputeUnits, utils.ErrBytes(err), nil, nil
	}
	wm := &warp.UnsignedMessage{
		// NetworkID + SourceChainID is populated by hypersdk
		Payload: payload,
	}
	return true, ExportAssetComputeUnits, nil, wm, nil
}

func (e *ExportAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	auth chain.Auth,
	txID ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if e.Value == 0 {
		return false, ExportAssetComputeUnits, OutputValueZero, nil, nil
	}
	if e.Destination == ids.Empty {
		// This would result in multiplying balance export by whoever imports the
		// transaction.
		return false, ExportAssetComputeUnits, OutputAnycast, nil, nil
	}
	// TODO: check if destination is ourselves
	if e.Return {
		return e.executeReturn(ctx, mu, auth.Actor(), txID)
	}
	return e.executeLoan(ctx, mu, auth.Actor(), txID)
}

func (*ExportAsset) MaxComputeUnits(chain.Rules) uint64 {
	return ExportAssetComputeUnits
}

func (*ExportAsset) Size() int {
	return codec.AddressLen + consts.IDLen +
		consts.Uint64Len + consts.BoolLen +
		consts.Uint64Len + /* op bits */
		consts.Uint64Len + consts.Uint64Len + consts.IDLen + consts.Uint64Len +
		consts.Int64Len + consts.IDLen
}

func (e *ExportAsset) Marshal(p *codec.Packer) {
	p.PackAddress(e.To)
	p.PackID(e.Asset)
	p.PackUint64(e.Value)
	p.PackBool(e.Return)
	op := codec.NewOptionalWriter(consts.Uint64Len*3 + consts.Int64Len + consts.IDLen)
	op.PackUint64(e.Reward)
	op.PackUint64(e.SwapIn)
	op.PackID(e.AssetOut)
	op.PackUint64(e.SwapOut)
	op.PackInt64(e.SwapExpiry)
	p.PackOptional(op)
	p.PackID(e.Destination)
}

func UnmarshalExportAsset(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var export ExportAsset
	p.UnpackAddress(&export.To)
	p.UnpackID(false, &export.Asset) // may export native
	export.Value = p.UnpackUint64(true)
	export.Return = p.UnpackBool()
	op := p.NewOptionalReader()
	export.Reward = op.UnpackUint64() // reward not required
	export.SwapIn = op.UnpackUint64() // optional
	op.UnpackID(&export.AssetOut)
	export.SwapOut = op.UnpackUint64()
	export.SwapExpiry = op.UnpackInt64()
	op.Done()
	p.UnpackID(true, &export.Destination)
	if err := p.Err(); err != nil {
		return nil, err
	}
	// Handle swap checks
	if !ValidSwapParams(
		export.Value,
		export.SwapIn,
		export.AssetOut,
		export.SwapOut,
		export.SwapExpiry,
	) {
		return nil, chain.ErrInvalidObject
	}
	return &export, nil
}

func (*ExportAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
