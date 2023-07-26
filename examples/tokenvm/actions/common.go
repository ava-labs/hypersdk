// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

const (
	// IDs explicitly listed. Registry will avoid usage of duplicated IDs by returning an error when occurs
	burnAssetID   uint8 = 0
	closeOrderID  uint8 = 1
	createAssetID uint8 = 2
	exportAssetID uint8 = 3
	importAssetID uint8 = 4
	createOrderID uint8 = 5
	fillOrderID   uint8 = 6
	mintAssetID   uint8 = 7
	modifyAssetID uint8 = 8
	transferID    uint8 = 9
)
