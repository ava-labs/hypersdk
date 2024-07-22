// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

const (
	// Action TypeIDs
	TransferID uint8 = 0

	// Custom NFT-VM Action Types
	CreateNFTCollection            uint8 = 1
	CreateNFTInstance              uint8 = 2
	CreateMarketplaceOrder         uint8 = 3
	BuyNFT                         uint8 = 4
	TransferNFTCollectionOwnership uint8 = 5
	TransferNFTInstance            uint8 = 6

	// Auth TypeIDs
	ED25519ID   uint8 = 0
	SECP256R1ID uint8 = 1
	BLSID       uint8 = 2

	// Relating to NFT Address generation
	NFTCOLLECTIONID uint8 = 3
)
