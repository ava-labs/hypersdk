// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import "errors"

// Token-related errors
var (
	ErrOutputValueZero                = errors.New("value is zero")
	ErrOutputTokenNameEmpty           = errors.New("token name is empty")
	ErrOutputTokenNameTooLarge        = errors.New("token name is too large")
	ErrOutputForbiddenTokenName       = errors.New("forbidden token name")
	ErrOutputTokenSymbolEmpty         = errors.New("token symbol is empty")
	ErrOutputTokenSymbolTooLarge      = errors.New("token symbol is too large")
	ErrOutputTokenMetadataEmpty       = errors.New("token metadata is empty")
	ErrOutputTokenMetadataTooLarge    = errors.New("token metadata is too large")
	ErrOutputTokenDecimalsZero        = errors.New("token decimals is zero")
	ErrOutputTokenDecimalsTooPrecise  = errors.New("token decimals are too precise")
	ErrOutputTokenAlreadyExists       = errors.New("token already exists")
	ErrOutputTokenDoesNotExist        = errors.New("token does not exist")
	ErrOutputTokenNotOwner            = errors.New("actor is not token owner")
	ErrOutputMintValueZero            = errors.New("mint value is zero")
	ErrOutputBurnValueZero            = errors.New("burn value is zero")
	ErrOutputTransferValueZero        = errors.New("transfer value is zero")
	ErrOutputInsufficientTokenBalance = errors.New("insufficient tokeån balance")
)

// Liquidity Pool related errors
var (
	ErrOutputInvalidFee                  = errors.New("proposed fee is not between 0 and 100")
	ErrOutputTokenXDoesNotExist          = errors.New("token X does not exist")
	ErrOutputTokenYDoesNotExist          = errors.New("token Y does not exist")
	ErrOutputFunctionDoesNotExist        = errors.New("function does not exist")
	ErrOutputIdenticalTokens             = errors.New("token X and token Y are identical")
	ErrOutputLiquidityPoolAlreadyExists  = errors.New("liquidity pool already exists")
	ErrOutputLiquidityPoolDoesNotExist   = errors.New("liquidity pool does not exist")
	ErrOutputLiquidityPoolEmptyReserves  = errors.New("liquidity pool reserves are empty")
	ErrOutputInsufficientLiquidityMinted = errors.New("insufficient liquidity minted")
	ErrOutputInsufficientLiquidityBurned = errors.New("insufficient liquidity burned")
	ErrOutputFeeLogicNotImplemented      = errors.New("fee logic not implemented")
)
