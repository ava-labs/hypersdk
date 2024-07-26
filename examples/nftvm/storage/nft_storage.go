// Logic relevant to reading/writing NFTs to storage
package storage

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	mconsts "github.com/ava-labs/hypersdk/examples/nftvm/consts"
)

// [Collection Prefix] + [Collection Address] + [Collection Chunks]
func CollectionStateKey(addr codec.Address) []byte {
	k := make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = nftCollectionPrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], nftCollectionStateChunks)
	return k
}

func InstanceStateKey(collectionAddr codec.Address, instanceNum uint32) []byte {
	k := make([]byte, 1+codec.AddressLen+consts.Uint32Len+consts.Uint16Len)
	k[0] = nftInstancePrefix
	copy(k[1:], collectionAddr[:])
	binary.BigEndian.PutUint32(k[1+codec.AddressLen:], instanceNum)
	binary.BigEndian.PutUint16(k[1+codec.AddressLen+consts.Uint32Len:], nftInstanceStateChunks)
	return k
}

func GenerateNFTCollectionAddress(name []byte, symbol []byte, metadata []byte) codec.Address {
	v := make([]byte, len(name)+len(symbol)+len(metadata))
	copy(v, name)
	copy(v[len(name):], symbol)
	copy(v[len(name)+len(symbol):], metadata)
	id := utils.ToID(v)
	return codec.CreateAddress(mconsts.NFTCOLLECTIONID, id)
}

// Writes NFT Collection to state
func SetNFTCollection(
	ctx context.Context,
	mu state.Mutable,
	collectionAddress codec.Address,
	name []byte,
	symbol []byte,
	metadata []byte,
	numOfInstances uint32,
	owner codec.Address,
) error {
	collectionStateKey := CollectionStateKey(collectionAddress)

	nameLen := len(name)
	symbolLen := len(symbol)
	metadataLen := len(metadata)

	v := make([]byte, consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen+consts.Uint32Len+codec.AddressLen)

	// Insert name
	binary.BigEndian.PutUint16(v, uint16(nameLen))
	copy(v[consts.Uint16Len:], name)

	// Insert symbol
	binary.BigEndian.PutUint16(v[consts.Uint16Len+nameLen:], uint16(symbolLen))
	copy(v[consts.Uint16Len+nameLen+consts.Uint16Len:], symbol)

	// Insert metadata
	binary.BigEndian.PutUint16(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen:], uint16(metadataLen))
	copy(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len:], metadata)

	// Insert number of instances
	binary.BigEndian.PutUint32(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen:], numOfInstances)

	// Insert collection owner
	copy(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen+consts.Uint32Len:], owner[:])

	return mu.Insert(ctx, collectionStateKey, v)
}

func GetNFTCollection(
	ctx context.Context,
	f ReadState,
	collectionAddress codec.Address,
) ([]byte, []byte, []byte, uint32, codec.Address, error) {
	k := CollectionStateKey(collectionAddress)
	values, errs := f(ctx, [][]byte{k})
	if errs[0] != nil {
		return []byte{}, []byte{}, []byte{}, 0, codec.EmptyAddress, errs[0]
	}
	return innerGetNFTCollection(values[0], errs[0])
}

// To be used when we want to get the state of an NFT collection during an
// action execution (i.e. not via JSON-RPC method)
func GetNFTCollectionNoController(
	ctx context.Context,
	mu state.Mutable,
	collectionAddress codec.Address,
) ([]byte, []byte, []byte, uint32, codec.Address, error) {
	value, err := mu.GetValue(ctx, CollectionStateKey(collectionAddress))
	if err != nil {
		return []byte{}, []byte{}, []byte{}, 0, codec.EmptyAddress, err
	}
	return innerGetNFTCollection(value, err)
}

func innerGetNFTCollection(
	v []byte,
	err error,
) ([]byte, []byte, []byte, uint32, codec.Address, error) {
	// Extract name
	nameLen := binary.BigEndian.Uint16(v)
	name := v[consts.Uint16Len : consts.Uint16Len+nameLen]
	// Extract symbol
	symbolLen := binary.BigEndian.Uint16(v[consts.Uint16Len+nameLen:])
	symbol := v[consts.Uint16Len+nameLen+consts.Uint16Len : consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen]
	// Extract metadata
	metadataLen := binary.BigEndian.Uint16(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen:])
	metadata := v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len : consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen]
	// Extract numOfInstances
	numOfInstances := binary.BigEndian.Uint32(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen:])
	// Extract owner
	owner := codec.Address(v[consts.Uint16Len+nameLen+consts.Uint16Len+symbolLen+consts.Uint16Len+metadataLen+consts.Uint32Len:])
	return name, symbol, metadata, numOfInstances, owner, nil
}

// This function takes care of the following
// 1. Creating a NFT Instance by assigning it the next available instance number
// 2. Updating the parent collection state (i.e. by incrementing numOfInstances)
func CreateNFTInstance(
	ctx context.Context,
	mu state.Mutable,
	collectionAddress codec.Address,
	owner codec.Address,
	metadata []byte,
) (uint32, error) {
	// Get parent collection state
	name, symbol, collectionMetadata, numOfInstances, collectionOwner, err := GetNFTCollectionNoController(ctx, mu, collectionAddress)
	if err != nil {
		return 0, err
	}

	// Write instance to state
	if err = SetNFTInstance(ctx, mu, collectionAddress, numOfInstances, owner, metadata, false); err != nil {
		return 0, err
	}

	// Update parent collection in state
	if err = SetNFTCollection(ctx, mu, collectionAddress, name, symbol, collectionMetadata, numOfInstances+1, collectionOwner); err != nil {
		return 0, nil
	}

	return numOfInstances, nil
}

// Writes NFT instance to state
func SetNFTInstance(
	ctx context.Context,
	mu state.Mutable,
	collectionAddress codec.Address,
	instanceNum uint32,
	owner codec.Address,
	metadata []byte,
	isListedOnMarketplace bool,
) error {
	instanceStateKey := InstanceStateKey(collectionAddress, instanceNum)

	metadataLen := len(metadata)
	v := make([]byte, codec.AddressLen+consts.Uint16Len+metadataLen+consts.Uint16Len)

	// Insert instance owner
	copy(v, owner[:])

	// Insert metadata
	binary.BigEndian.PutUint16(v[codec.AddressLen:], uint16(metadataLen))
	copy(v[codec.AddressLen+consts.Uint16Len:], metadata)

	// Insert marketplace indicator
	if isListedOnMarketplace {
		binary.BigEndian.PutUint16(v[codec.AddressLen+consts.Uint16Len+metadataLen:], ListedOnMarketplace)
	} else {
		binary.BigEndian.PutUint16(v[codec.AddressLen+consts.Uint16Len+metadataLen:], NotListedOnMarketplace)
	}

	return mu.Insert(ctx, instanceStateKey, v)
}

func GetNFTInstance(
	ctx context.Context,
	f ReadState,
	collectionAddress codec.Address,
	instanceNum uint32,
) (codec.Address, []byte, bool, error) {
	k := InstanceStateKey(collectionAddress, instanceNum)
	values, errs := f(ctx, [][]byte{k})
	if errs[0] != nil {
		return codec.EmptyAddress, []byte{}, false, errs[0]
	}
	return innerGetNFTInstance(values[0], errs[0])
}

func GetNFTInstanceNoController(
	ctx context.Context,
	mu state.Mutable,
	collectionAddress codec.Address,
	instanceNum uint32,
) (codec.Address, []byte, bool, error) {
	value, err := mu.GetValue(ctx, InstanceStateKey(collectionAddress, instanceNum))
	if err != nil {
		return codec.EmptyAddress, []byte{}, false, err
	}
	return innerGetNFTInstance(value, err)
}

func innerGetNFTInstance(
	v []byte,
	err error,
) (codec.Address, []byte, bool, error) {
	// Extract owner
	owner := codec.Address(v[:codec.AddressLen])
	// Extract metadata
	metadataLen := binary.BigEndian.Uint16(v[codec.AddressLen:])
	metadata := v[codec.AddressLen+consts.Uint16Len : codec.AddressLen+consts.Uint16Len+metadataLen]
	// Extract marketplace indicator
	marketplaceIndicator := binary.BigEndian.Uint16(v[codec.AddressLen+consts.Uint16Len+metadataLen:])
	var isListedOnMarketplace bool

	switch marketplaceIndicator {
	case ListedOnMarketplace:
		isListedOnMarketplace = true
	case NotListedOnMarketplace:
		isListedOnMarketplace = false
	default:
		return codec.EmptyAddress, []byte{}, false, ErrCorruptInstanceMarketplaceIndicator
	}

	return owner, metadata, isListedOnMarketplace, nil
}
