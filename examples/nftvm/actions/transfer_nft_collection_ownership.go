package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*TransferNFTCollectionOwnership)(nil)

type TransferNFTCollectionOwnership struct {
	CollectionAddress codec.Address `json:"collectionAddress"`
	To                codec.Address `json:"to"`
}

// ComputeUnits implements chain.Action.
func (t *TransferNFTCollectionOwnership) ComputeUnits(chain.Rules) uint64 {
	return TransferNFTCollectionComputeUnits
}

// Execute implements chain.Action.
func (t *TransferNFTCollectionOwnership) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Assert that collection exists
	name, symbol, metadata, numOfInstances, owner, err := storage.GetNFTCollectionNoController(ctx, mu, t.CollectionAddress)
	if err != nil {
		return nil, ErrOutputNFTCollectionDoesNotExist
	}
	// Assert that actor == current collection owner
	if actor != owner {
		return nil, ErrOutputNFTCollectionNotOwner
	}

	// Finish transfer by updating NFT Collection in state
	if err = storage.SetNFTCollection(ctx, mu, t.CollectionAddress, name, symbol, metadata, numOfInstances, t.To); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (t *TransferNFTCollectionOwnership) GetTypeID() uint8 {
	return consts.TransferNFTCollectionOwnership
}

// Size implements chain.Action.
func (t *TransferNFTCollectionOwnership) Size() int {
	return codec.AddressLen + codec.AddressLen
}

// StateKeys implements chain.Action.
func (t *TransferNFTCollectionOwnership) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.CollectionStateKey(t.CollectionAddress)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (t *TransferNFTCollectionOwnership) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.MaxCollectionSize}
}

// ValidRange implements chain.Action.
func (t *TransferNFTCollectionOwnership) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// Marshal implements chain.Action.
func (t *TransferNFTCollectionOwnership) Marshal(p *codec.Packer) {
	p.PackAddress(t.CollectionAddress)
	p.PackAddress(t.To)
}

func UnmarshalTransferNFTCollectionOwnership(p *codec.Packer) (chain.Action, error) {
	var transferNFTCollectionOwnership TransferNFTCollectionOwnership

	p.UnpackAddress(&transferNFTCollectionOwnership.CollectionAddress)
	p.UnpackAddress(&transferNFTCollectionOwnership.To)

	return &transferNFTCollectionOwnership, p.Err()
}
