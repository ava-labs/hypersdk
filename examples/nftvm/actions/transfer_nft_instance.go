package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"

	lconsts "github.com/ava-labs/hypersdk/consts"
)

var _ chain.Action = (*TransferNFTInstance)(nil)

type TransferNFTInstance struct {
	CollectionAddress codec.Address `json:"collectionAddress"`
	InstanceNum       uint32        `json:"instanceNum"`
	To                codec.Address `json:"to"`
}

// ComputeUnits implements chain.Action.
func (t *TransferNFTInstance) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

// Execute implements chain.Action.
func (t *TransferNFTInstance) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
	// Assert that collection exists
	owner, metadata, err := storage.GetNFTInstanceNoController(ctx, mu, t.CollectionAddress, t.InstanceNum)
	if err != nil {
		return nil, ErrOutputNFTInstanceNotFound
	}
	// Assert that actor is current instance owner
	if owner != actor {
		return nil, ErrOutputNFTInstanceNotOwner
	}
	// Finish transfer by updating NFT instance in state
	if err = storage.SetNFTInstance(ctx, mu, t.CollectionAddress, t.InstanceNum, t.To, metadata); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (t *TransferNFTInstance) GetTypeID() uint8 {
	return consts.TransferNFTInstance
}

// Marshal implements chain.Action.
func (t *TransferNFTInstance) Marshal(p *codec.Packer) {
	p.PackAddress(t.CollectionAddress)
	p.PackInt(int(t.InstanceNum))
	p.PackAddress(t.To)
}

// Size implements chain.Action.
func (t *TransferNFTInstance) Size() int {
	return codec.AddressLen + lconsts.Uint32Len + codec.AddressLen
}

// StateKeys implements chain.Action.
func (t *TransferNFTInstance) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.InstanceStateKey(t.CollectionAddress, t.InstanceNum)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (t *TransferNFTInstance) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.MaxInstanceSize}
}

// ValidRange implements chain.Action.
func (t *TransferNFTInstance) ValidRange(chain.Rules) (start int64, end int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func UnmarshalTransferNFTInstance(p *codec.Packer) (chain.Action, error) {
	var transferNFTInstance TransferNFTInstance

	p.UnpackAddress(&transferNFTInstance.CollectionAddress)
	transferNFTInstance.InstanceNum = uint32(p.UnpackInt(false))
	p.UnpackAddress(&transferNFTInstance.To)

	return &transferNFTInstance, p.Err()
}