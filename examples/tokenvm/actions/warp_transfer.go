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

const WarpTransferSize = crypto.PublicKeyLen + 2*consts.IDLen + 2*consts.Uint64Len + 1

type WarpTransfer struct {
	To     crypto.PublicKey `json:"to"`
	Asset  ids.ID           `json:"asset"`
	Value  uint64           `json:"value"`
	Return bool             `json:"return"`

	Reward uint64 `json:"reward"`

	TxID ids.ID `json:"txID"`
}

func (w *WarpTransfer) Marshal() ([]byte, error) {
	p := codec.NewWriter(WarpTransferSize)
	p.PackPublicKey(w.To)
	p.PackID(w.Asset)
	p.PackUint64(w.Value)
	p.PackBool(w.Return)
	p.PackUint64(w.Reward)
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
	transfer.Reward = p.UnpackUint64(false) // reward not required
	p.UnpackID(true, &transfer.TxID)
	if err := p.Err(); err != nil {
		return nil, err
	}
	if !p.Empty() {
		return nil, chain.ErrInvalidObject
	}
	return &transfer, nil
}
