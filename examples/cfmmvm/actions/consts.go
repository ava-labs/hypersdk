// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

// Compute units for actions
// TODO: fine tune all of these values
const (
	// Relating to tokens
	CreateTokenComputeUnits   = 1
	MintTokenComputeUnits     = 1
	BurnTokenComputeUnits     = 1
	TransferTokenComputeUnits = 1

	// Relating to LPs
	CreateLiquidityPoolUnits = 1
	AddLiquidityUnits        = 1
	RemoveLiquidityUnits     = 1
	SwapUnits                = 1

	GetTokenInfoUnits           = 1
	GetTokenAccountBalanceUnits = 1
	GetLiquidityPoolInfoUnits   = 1
)
