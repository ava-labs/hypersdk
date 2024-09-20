// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/codec"
)

// TypeIDs for actions
const (
	// Token-related
	CreateTokenID uint8 = iota
	MintTokenID
	BurnTokenID
	TransferTokenID
	// LP-related
	CreateLiquidityPoolID
	AddLiquidityID
	RemoveLiquidityID
	SwapID

	// Read-only actions
	GetTokenInfoID
	GetTokenAccountBalanceID
	GetLiquidityPoolInfoID
)

// TypeIDs for auth
const (
	// Required
	ED25519ID uint8 = iota
	SECP256R1ID
	BLSID

	// Relating to various CFMM-VM address generation
	TOKENID
	LIQUIDITYPOOLID
	LIQUIDITYPOOLTOKENID
)

const (
	Name = "CFMMVM"
	HRP  = "cfmm"
)

var (
	ID          ids.ID
	CoinAddress codec.Address
)

func init() {
	b := make([]byte, ids.IDLen)
	copy(b, []byte(Name))
	vmID, err := ids.ToID(b)
	if err != nil {
		panic(err)
	}
	ID = vmID
}

var Version = &version.Semantic{
	Major: 0,
	Minor: 0,
	Patch: 1,
}
