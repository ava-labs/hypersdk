// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
)

const WarpTransferSize = crypto.PublicKeyLen + consts.IDLen +
	consts.Uint64Len + 1 + consts.Uint64Len + consts.Uint64Len +
	consts.IDLen + consts.Uint64Len + consts.Uint64Len + consts.IDLen

type WarpTransfer struct {
	To    crypto.PublicKey `json:"to"`
	Asset ids.ID           `json:"asset"`
	Value uint64           `json:"value"`

	// Return is set to true when a warp message is sending funds back to the
	// chain where they were created.
	Return bool `json:"return"`

	// Reward is the amount of [Asset] to send the [Actor] that submits this
	// transaction.
	Reward uint64 `json:"reward"`

	// SwapIn is the amount of [Asset] we are willing to swap for [AssetOut].
	SwapIn uint64 `json:"swapIn"`
	// AssetOut is the asset we are seeking to get for [SwapIn].
	AssetOut ids.ID `json:"assetOut"`
	// SwapOut is the amount of [AssetOut] we are seeking.
	SwapOut uint64 `json:"swapOut"`
	// SwapExpiry is the unix timestamp at which the swap becomes invalid (and
	// the message can be processed without a swap.
	SwapExpiry int64 `json:"swapExpiry"`

	// TxID is the transaction that created this message. This is used to ensure
	// there is WarpID uniqueness.
	TxID ids.ID `json:"txID"`
}

func (w *WarpTransfer) Marshal() ([]byte, error) {
	p := codec.NewWriter(WarpTransferSize)
	p.PackPublicKey(w.To)
	p.PackID(w.Asset)
	p.PackUint64(w.Value)
	p.PackBool(w.Return)
	op := codec.NewOptionalWriter()
	op.PackUint64(w.Reward)
	op.PackUint64(w.SwapIn)
	op.PackID(w.AssetOut)
	op.PackUint64(w.SwapOut)
	op.PackInt64(w.SwapExpiry)
	p.PackOptional(op)
	p.PackID(w.TxID)
	return p.Bytes(), p.Err()
}

func ImportedAssetID(assetID ids.ID, sourceChainID ids.ID) ids.ID {
	return utils.ToID(ImportedAssetMetadata(assetID, sourceChainID))
}

func ImportedAssetMetadata(assetID ids.ID, sourceChainID ids.ID) []byte {
	k := make([]byte, consts.IDLen*2)
	copy(k, assetID[:])
	copy(k[consts.IDLen:], sourceChainID[:])
	return k
}

func UnmarshalWarpTransfer(b []byte) (*WarpTransfer, error) {
	var transfer WarpTransfer
	p := codec.NewReader(b, WarpTransferSize)
	p.UnpackPublicKey(false, &transfer.To)
	p.UnpackID(false, &transfer.Asset)
	transfer.Value = p.UnpackUint64(true)
	transfer.Return = p.UnpackBool()
	op := p.NewOptionalReader()
	transfer.Reward = op.UnpackUint64() // reward not required
	transfer.SwapIn = op.UnpackUint64() // optional
	op.UnpackID(&transfer.AssetOut)
	transfer.SwapOut = op.UnpackUint64()
	transfer.SwapExpiry = op.UnpackInt64()
	op.Done()
	p.UnpackID(true, &transfer.TxID)
	if err := p.Err(); err != nil {
		return nil, err
	}
	if !p.Empty() {
		return nil, chain.ErrInvalidObject
	}
	// Handle swap checks
	if !ValidSwapParams(
		transfer.Value,
		transfer.SwapIn,
		transfer.AssetOut,
		transfer.SwapOut,
		transfer.SwapExpiry,
	) {
		return nil, chain.ErrInvalidObject
	}
	return &transfer, nil
}

func ValidSwapParams(
	value uint64,
	swapIn uint64,
	assetOut ids.ID,
	swapOut uint64,
	swapExpiry int64,
) bool {
	if swapExpiry < 0 {
		return false
	}
	if swapIn > value {
		return false
	}
	if swapIn > 0 {
		return swapOut != 0
	}
	if assetOut != ids.Empty {
		return false
	}
	if swapOut != 0 {
		return false
	}
	if swapExpiry != 0 {
		return false
	}
	return true
}
