// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import "errors"

var (
	ErrOutputValueZero          = errors.New("value is zero")
	ErrOutputMemoTooLarge       = errors.New("memo is too large")
	ErrOutputAssetIsNative      = errors.New("cannot mint native asset")
	ErrOutputAssetAlreadyExists = errors.New("asset already exists")
	ErrOutputAssetMissing       = errors.New("asset missing")
	ErrOutputInTickZero         = errors.New("in rate is zero")
	ErrOutputOutTickZero        = errors.New("out rate is zero")
	ErrOutputSupplyZero         = errors.New("supply is zero")
	ErrOutputSupplyMisaligned   = errors.New("supply is misaligned")
	ErrOutputOrderMissing       = errors.New("order is missing")
	ErrOutputUnauthorized       = errors.New("unauthorized")
	ErrOutputWrongIn            = errors.New("wrong in asset")
	ErrOutputWrongOut           = errors.New("wrong out asset")
	ErrOutputWrongOwner         = errors.New("wrong owner")
	ErrOutputInsufficientInput  = errors.New("insufficient input")
	ErrOutputInsufficientOutput = errors.New("insufficient output")
	ErrOutputValueMisaligned    = errors.New("value is misaligned")
	ErrOutputSymbolEmpty        = errors.New("symbol is empty")
	ErrOutputSymbolIncorrect    = errors.New("symbol is incorrect")
	ErrOutputSymbolTooLarge     = errors.New("symbol is too large")
	ErrOutputDecimalsIncorrect  = errors.New("decimal is incorrect")
	ErrOutputDecimalsTooLarge   = errors.New("decimal is too large")
	ErrOutputMetadataEmpty      = errors.New("metadata is empty")
	ErrOutputMetadataTooLarge   = errors.New("metadata is too large")
	ErrOutputSameInOut          = errors.New("same asset used for in and out")
	ErrOutputWrongDestination   = errors.New("wrong destination")
	ErrOutputMustFill           = errors.New("must fill request")
	ErrOutputInvalidDestination = errors.New("invalid destination")
)
