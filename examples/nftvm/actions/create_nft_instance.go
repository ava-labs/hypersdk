package actions

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"

	lconsts "github.com/ava-labs/hypersdk/consts"
)

var _ chain.Action = (*CreateNFTInstance)(nil)

type CreateNFTInstance struct {
	Owner codec.Address `json:"owner"`

	ParentCollection codec.Address `json:"parentCollection"`

	Metadata []byte `json:"metadata"`
}

// ComputeUnits implements chain.Action.
func (c *CreateNFTInstance) ComputeUnits(chain.Rules) uint64 {
	return CreateNFTInstanceComputeUnits
}

// Execute implements chain.Action.
func (c *CreateNFTInstance) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Enforce size invariants
	if len(c.Metadata) == 0 {
		return nil, ErrOutputInstanceMetadataEmpty
	}
	if len(c.Metadata) > storage.MaxInstanceMetadataSize {
		return nil, ErrOutputInstanceMetadataTooLarge
	}

	// Assert that collection exists
	_, _, _, _, collectionOwner, err := storage.GetNFTCollectionNoController(ctx, mu, c.ParentCollection)
	if err != nil {
		return nil, ErrOutputNFTCollectionDoesNotExist
	}

	// Only collection owners can mint NFTs
	if collectionOwner != actor {
		return nil, ErrOutputNFTCollectionNotOwner
	}

	// Finish execute
	instanceNum, err := storage.CreateNFTInstance(ctx, mu, c.ParentCollection, c.Owner, c.Metadata)
	if err != nil {
		return nil, err
	}

	v := make([]byte, lconsts.Uint32Len)
	binary.BigEndian.PutUint32(v, instanceNum)

	// Collection exists, delegate to storage
	return [][]byte{v}, nil
}

// GetTypeID implements chain.Action.
func (c *CreateNFTInstance) GetTypeID() uint8 {
	return consts.CreateNFTInstance
}

// Size implements chain.Action.
func (c *CreateNFTInstance) Size() int {
	return codec.AddressLen + codec.AddressLen + len(c.Metadata)
}

// StateKeys implements chain.Action.
func (c *CreateNFTInstance) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	// Action creates new instance in state while also modifying parent
	// collection
	parentCollectionStateKey := storage.CollectionStateKey(c.ParentCollection)
	// TODO: Remove hardcoded instance number
	instanceStateKey := storage.InstanceStateKey(c.ParentCollection, 0)
	return state.Keys{
		string(parentCollectionStateKey): state.All,
		string(instanceStateKey):         state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (c *CreateNFTInstance) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.MaxCollectionSize, storage.MaxInstanceSize}
}

// ValidRange implements chain.Action.
func (c *CreateNFTInstance) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// Marshal implements chain.Action.
func (c *CreateNFTInstance) Marshal(p *codec.Packer) {
	p.PackAddress(c.Owner)
	p.PackAddress(c.ParentCollection)
	p.PackBytes(c.Metadata)
}

func UnmarshalCreateNFTInstance(p *codec.Packer) (chain.Action, error) {
	var createNFTInstance CreateNFTInstance

	p.UnpackAddress(&createNFTInstance.Owner)
	p.UnpackAddress(&createNFTInstance.ParentCollection)
	p.UnpackBytes(storage.MaxInstanceMetadataSize, true, &createNFTInstance.Metadata)

	return &createNFTInstance, p.Err()
}
