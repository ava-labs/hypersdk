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

var _ chain.Action = (*CreateNFTCollection)(nil)

type CreateNFTCollection struct {

	Name []byte `json:"name"`

	Symbol   []byte `json:"symbol"`
	
	// Supplementary information about the collection
	Metadata []byte `json:"metadata"`
}

// ComputeUnits implements chain.Action.
func (c *CreateNFTCollection) ComputeUnits(chain.Rules) uint64 {
	return CreateNFTCollectionComputeUnits
}

// Execute implements chain.Action.
func (c *CreateNFTCollection) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
	// Enforce size invariants
	// TODO: put assertions into separate function
	if len(c.Name) == 0 {
		return nil, ErrOutputCollectionNameEmpty
	}
	if len(c.Symbol) == 0 {
		return nil, ErrOutputCollectionSymbolEmpty
	}
	if len(c.Metadata) == 0 {
		return nil, ErrOutputCollectionMetadataEmpty
	}

	if len(c.Name) > storage.MaxCollectionNameSize {
		return nil, ErrOutputCollectionNameTooLarge
	}
	if len(c.Symbol) > storage.MaxCollectionSymbolSize {
		return nil, ErrOutputCollectionSymbolTooLarge
	}
	if len(c.Metadata) > storage.MaxCollectionMetadataSize {
		return nil, ErrOutputCollectionMetadataTooLarge
	}

	// Generate collection address
	collectionAddress := storage.GenerateNFTCollectionAddress(c.Name, c.Symbol, c.Metadata)
	collectionStateKey := storage.CollectionStateKey(collectionAddress)
	// Assert that collection does not already exist
	if _, err := mu.GetValue(ctx, collectionStateKey); err == nil {
		return nil, ErrOutputNFTCollectionAlreadyExists
	}
	
	if err := storage.SetNFTCollection(ctx, mu, collectionAddress, c.Name, c.Symbol, c.Metadata, 0, actor); err != nil {
		return nil, err
	}

	return [][]byte{collectionAddress[:]}, nil
}

// GetTypeID implements chain.Action.
func (c *CreateNFTCollection) GetTypeID() uint8 {
	return consts.CreateNFTCollection
}

// Marshal implements chain.Action.
func (c *CreateNFTCollection) Marshal(p *codec.Packer) {
	p.PackBytes(c.Name)
	p.PackBytes(c.Symbol)
	p.PackBytes(c.Metadata)
}

// Size implements chain.Action.
func (c *CreateNFTCollection) Size() int {
	return len(c.Name) + len(c.Symbol) + len(c.Metadata)
}

// StateKeys implements chain.Action.
func (c *CreateNFTCollection) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	collectionAddress := storage.GenerateNFTCollectionAddress(c.Name, c.Symbol, c.Metadata)
	return state.Keys{
		string(storage.CollectionStateKey(collectionAddress)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (c *CreateNFTCollection) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.MaxCollectionSize}
}

// ValidRange implements chain.Action.
func (c *CreateNFTCollection) ValidRange(chain.Rules) (start int64, end int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func UnmarshalCreateNFTCollection(p *codec.Packer) (chain.Action, error) {
	var createNFTCollection CreateNFTCollection

	p.UnpackBytes(storage.MaxCollectionNameSize, true, &createNFTCollection.Name)
	p.UnpackBytes(storage.MaxCollectionSymbolSize, true, &createNFTCollection.Symbol)
	p.UnpackBytes(storage.MaxCollectionMetadataSize, true, &createNFTCollection.Metadata)

	return &createNFTCollection, p.Err()
}
