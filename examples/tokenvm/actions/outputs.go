// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

var (
	OutputValueZero              = []byte("value is zero")
	OutputAssetIsNative          = []byte("cannot mint native asset")
	OutputAssetAlreadyExists     = []byte("asset already exists")
	OutputAssetMissing           = []byte("asset missing")
	OutputInTickZero             = []byte("in rate is zero")
	OutputOutTickZero            = []byte("out rate is zero")
	OutputSupplyZero             = []byte("supply is zero")
	OutputSupplyMisaligned       = []byte("supply is misaligned")
	OutputOrderMissing           = []byte("order is missing")
	OutputUnauthorized           = []byte("unauthorized")
	OutputWrongIn                = []byte("wrong in asset")
	OutputWrongOut               = []byte("wrong out asset")
	OutputWrongOwner             = []byte("wrong owner")
	OutputInsufficientInput      = []byte("insufficient input")
	OutputInsufficientOutput     = []byte("insufficient output")
	OutputValueMisaligned        = []byte("value is misaligned")
	OutputMetadataTooLarge       = []byte("metadata is too large")
	OutputSameInOut              = []byte("same asset used for in and out")
	OutputConflictingAsset       = []byte("warp has same asset as another")
	OutputAnycast                = []byte("anycast output")
	OutputNotWarpAsset           = []byte("not warp asset")
	OutputWarpAsset              = []byte("warp asset")
	OutputWrongDestination       = []byte("wrong destination")
	OutputMustFill               = []byte("must fill request")
	OutputWarpVerificationFailed = []byte("warp verification failed")
)
