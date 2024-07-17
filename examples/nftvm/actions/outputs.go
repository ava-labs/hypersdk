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
)