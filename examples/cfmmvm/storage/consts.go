// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"math"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
)

// Key prefixes
const (
	// Required for StateManager
	heightPrefix byte = iota
	timestampPrefix
	feePrefix

	// Required for CFMMVM
	tokenInfoPrefix
	tokenAccountBalancePrefix
	liquidityPoolPrefix
)

// TODO: tune these values
// Chunks
const (
	TokenInfoChunks           uint16 = 2
	TokenAccountBalanceChunks uint16 = 1
	LiquidityPoolChunks       uint16 = 1
)

// Related to action invariants
const (
	MaxTokenNameSize     = 64
	MaxTokenSymbolSize   = 8
	MaxTokenMetadataSize = 256
	MaxTokenDecimals     = 18
)

// All LP tokens have the following data
const (
	LiquidityPoolTokenName     = "CFMM-Pair" // #nosec G101
	LiquidityPoolTokenSymbol   = "CFMMP"
	LiquidityPoolTokenDecimals = 9
	LiquidityPoolTokenMetadata = "A liquidity pool"
)

// Data for CFMM-VM Coin
const (
	Symbol   = "CVM"
	Metadata = "A constant-function market-maker VM implementation"
)

var (
	MinimumLiquidity = uint64(math.Pow10(3))
	CoinAddress      codec.Address
)

func init() {
	CoinAddress = TokenAddress([]byte(consts.Name), []byte(Symbol), []byte(Metadata))
}
