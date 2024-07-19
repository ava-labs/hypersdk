// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import "errors"

var ErrOutputValueZero = errors.New("value is zero")

var (
	ErrOutputCollectionNameEmpty = errors.New("collection name is empty")
	ErrOutputCollectionSymbolEmpty = errors.New("collection symbol is empty")
	ErrOutputCollectionMetadataEmpty = errors.New("collection metdata is empty")

	ErrOutputCollectionNameTooLarge = errors.New("collection name is too large")
	ErrOutputCollectionSymbolTooLarge = errors.New("collection symbol is too large")
	ErrOutputCollectionMetadataTooLarge = errors.New("collection metadata is too large")

	ErrOutputInstanceMetadataEmpty = errors.New("instance name is empty")
	ErrOutputInstanceMetadataTooLarge = errors.New("instance metadata is too large")

	ErrOutputMarketplaceOrderPriceZero = errors.New("price is zero")
	ErrOutputMarketplaceOrderNotOwner = errors.New("actor is not NFT instance owner")
	ErrOutputMarketplaceOrderInsufficientBalance = errors.New("insufficient balance for purchase")
	ErrOutputMarketplaceOrderNotFound = errors.New("order not found")

	ErrOutputNFTCollectionAlreadyExists = errors.New("NFT collection already exists")
	ErrOutputNFTCollectionDoesNotExist = errors.New("NFT collection does not exist")
	ErrOutputNFTCollectionNotOwner = errors.New("not NFT collection owner")

	ErrOutputNFTInstanceNotFound = errors.New("NFT instance not found")
	ErrOutputNFTInstanceNotOwner = errors.New("not NFT instance owner")
	ErrOutputMarketplaceOrderOwnerInconsistency = errors.New("current instance owner and provided owner do not match")
	ErrOutputMarketplaceOrderInstanceNumInconsistency = errors.New("current instance num and provided num do not match")
)