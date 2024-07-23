package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// Metadata
// 0x0/ (tx)
//   -> [txID] => timestamp
//
// State
// / (height) => store in root
//   -> [heightPrefix] => height
// 0x0/ (balance)
//   -> [owner] => balance
// 0x1/ (hypersdk-height)
// 0x2/ (hypersdk-timestamp)
// 0x3/ (hypersdk-fee)

const (
	// metaDB
	txPrefix = 0x0

	// stateDB
	balancePrefix   = 0x0
	heightPrefix    = 0x1
	timestampPrefix = 0x2
	feePrefix       = 0x3
	nftCollectionPrefix    = 0x4
	nftInstancePrefix      = 0x5
	marketplaceOrderPrefix = 0x6
)

const BalanceChunks uint16 = 1

var (
	failureByte  = byte(0x0)
	successByte  = byte(0x1)
	heightKey    = []byte{heightPrefix}
	timestampKey = []byte{timestampPrefix}
	feeKey       = []byte{feePrefix}
)

const (
	ListedOnMarketplace = uint16(iota)
	NotListedOnMarketplace = uint16(iota)
)

const (
	// Limits on collections
	MaxCollectionNameSize     = 64
	MaxCollectionSymbolSize   = 8
	MaxCollectionMetadataSize = 256

	// TODO: tune this
	MaxCollectionSize = MaxCollectionNameSize + MaxCollectionSymbolSize + MaxCollectionMetadataSize + consts.Uint32Len

	// Limits on instances
	MaxInstanceMetadataSize = 256

	MaxInstanceSize = codec.AddressLen + MaxCollectionMetadataSize
)

// Denominated in bytes
const nftCollectionStateChunks uint16 = MaxCollectionNameSize + MaxCollectionSymbolSize + MaxCollectionMetadataSize + consts.Uint16Len

const nftInstanceStateChunks uint16 = codec.AddressLen + MaxInstanceMetadataSize

const MarketplaceOrderStateChunks = consts.Uint64Len

// TODO: tune this
const BuyNFTStateChunks = 50