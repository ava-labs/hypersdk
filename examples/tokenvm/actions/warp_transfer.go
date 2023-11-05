// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

type WarpTransfer struct {
	To       codec.Address `json:"to"`
	Symbol   []byte        `json:"symbol"`
	Decimals uint8         `json:"decimals"`
	Asset    ids.ID        `json:"asset"`
	Value    uint64        `json:"value"`

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

	// DestinationChainID is the destination of this transfer. We assume this
	// must be populated (not anycast).
	DestinationChainID ids.ID `json:"destinationChainID"`
}

func (w *WarpTransfer) size() int {
	return codec.AddressLen + codec.BytesLen(w.Symbol) + consts.Uint8Len + consts.IDLen +
		consts.Uint64Len + consts.BoolLen +
		consts.Uint64Len + /* op bits */
		consts.Uint64Len + consts.Uint64Len + consts.IDLen + consts.Uint64Len + consts.Int64Len +
		consts.IDLen + consts.IDLen
}

func (w *WarpTransfer) Marshal() ([]byte, error) {
	p := codec.NewWriter(w.size(), w.size())
	p.PackAddress(w.To)
	p.PackBytes(w.Symbol)
	p.PackByte(w.Decimals)
	p.PackID(w.Asset)
	p.PackUint64(w.Value)
	p.PackBool(w.Return)
	op := codec.NewOptionalWriter(consts.Uint64Len*3 + consts.IDLen + consts.Int64Len)
	op.PackUint64(w.Reward)
	op.PackUint64(w.SwapIn)
	op.PackID(w.AssetOut)
	op.PackUint64(w.SwapOut)
	op.PackInt64(w.SwapExpiry)
	p.PackOptional(op)
	p.PackID(w.TxID)
	p.PackID(w.DestinationChainID)
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
	maxWarpTransferSize := codec.AddressLen + codec.BytesLenSize(MaxSymbolSize) + consts.Uint8Len + consts.IDLen +
		consts.Uint64Len + consts.BoolLen +
		consts.Uint64Len + /* op bits */
		consts.Uint64Len + consts.Uint64Len + consts.IDLen + consts.Uint64Len + consts.Int64Len +
		consts.IDLen + consts.IDLen

	var transfer WarpTransfer
	p := codec.NewReader(b, maxWarpTransferSize)
	p.UnpackAddress(&transfer.To)
	p.UnpackBytes(MaxSymbolSize, true, &transfer.Symbol)
	transfer.Decimals = p.UnpackByte()
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
	p.UnpackID(true, &transfer.DestinationChainID)
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
