package storage

import "math"

// Metadata
// 0x0/ (tx)
//   -> [txID] => timestamp
//
// State
// / (height) => store in root
//   -> [heightPrefix] => height
// 0x0/ (balance)
//   -> [owner] => balance
// 0x1/ (hypersdk-height)
// 0x2/ (hypersdk-timestamp)
// 0x3/ (hypersdk-fee)

const (
	// metaDB
	txPrefix = 0x0

	// stateDB
	balancePrefix   = 0x0
	heightPrefix    = 0x1
	timestampPrefix = 0x2
	feePrefix       = 0x3
	tokenInfoPrefix = 0x4
	tokenAccountInfoPrefix = 0x5
	liquidityPoolPrefix = 0x6
)

const (
	BalanceChunks uint16 = 1
	// TODO: tune this
	TokenInfoChunks uint16 = 2
	TokenAccountInfoChunks uint16 = 2
	LiquidityPoolChunks uint16 = 4
)

const (
	LiquidityPoolTokenName = "CFMM-Pair"
	LiquidityPoolTokenSymbol = "CFMMP"
	LiquidityPoolTokenDecimals = 18
	LiquidityPoolTokenMetadata = "A liquidity pool"
)

var MinimumLiquidity = math.Pow(10, 3)

var (
	failureByte  = byte(0x0)
	successByte  = byte(0x1)
	heightKey    = []byte{heightPrefix}
	timestampKey = []byte{timestampPrefix}
	feeKey       = []byte{feePrefix}
)