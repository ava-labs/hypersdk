// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pricing

import "errors"

var (
	ErrOutputFunctionDoesNotExist = errors.New("function does not exist")

	ErrReservesZero          = errors.New("reserves are zero")
	ErrBothDeltasZero        = errors.New("both delta values are equal to zero")
	ErrNoClearDeltaToCompute = errors.New("both delta values are nonzero")

	ErrOutputInsufficientLiquidityMinted = errors.New("insufficient liquidity minted")
)
