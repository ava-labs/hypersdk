package actions

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
)

const WarpTransferSize = crypto.PublicKeyLen + 2*consts.IDLen + 2*consts.Uint64Len

type WarpTransfer struct {
	To     crypto.PublicKey `json:"to"`
	Asset  ids.ID           `json:"asset"`
	Value  uint64           `json:"value"`
	Reward uint64           `json:"reward"`
	TxID   ids.ID           `json:"txID"`
}

func (w *WarpTransfer) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackPublicKey(w.To)
	p.PackID(w.Asset)
	p.PackUint64(w.Value)
	p.PackUint64(w.Reward)
	p.PackID(w.TxID)
	return p.Bytes(), p.Err()
}

func (w *WarpTransfer) NewAssetID(sourceChainID ids.ID) ids.ID {
	k := make([]byte, consts.IDLen*2)
	copy(k, w.Asset[:])
	copy(k[consts.IDLen:], sourceChainID[:])
	return utils.ToID(k)
}

func UnmarshalWarpTransfer(b []byte) (*WarpTransfer, error) {
	var transfer WarpTransfer
	p := codec.NewReader(b, WarpTransferSize)
	p.UnpackPublicKey(false, &transfer.To)
	p.UnpackID(false, &transfer.Asset)
	transfer.Value = p.UnpackUint64(true)
	transfer.Reward = p.UnpackUint64(false) // reward not required
	p.UnpackID(true, &transfer.TxID)
	if !p.Empty() {
		return nil, chain.ErrInvalidObject
	}
	return &transfer, p.Err()
}
