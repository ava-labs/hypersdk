// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import hactions "github.com/ava-labs/hypersdk/actions"

const (
	// Action TypeIDs
	TransferID       uint8 = 0
	MsgID            uint8 = 1
	AnchorRegisterID uint8 = hactions.AnchorRegisterID
	// Auth TypeIDs
	ED25519ID   uint8 = 0
	SECP256R1ID uint8 = 1
	BLSID       uint8 = 2
)
