// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

const (
	// Action TypeIDs
	TransferID uint8 = iota
	CreateTokenID
	MintTokenID
	BurnTokenID
	TransferTokenID
	CreateLiquidityPoolID
	DepositLiquidityID
	RemoveLiquidityID
	SwapID
)

const (
	// Auth TypeIDs
	ED25519ID uint8 = iota
	SECP256R1ID
	BLSID

	// Relating to various CFMM-VM address generation
	TOKENID
	LIQUIDITYPOOLID
	LIQUIDITYPOOLTOKENID
)
