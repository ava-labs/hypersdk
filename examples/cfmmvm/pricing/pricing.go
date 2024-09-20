// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pricing

// IDs for pricing models
const (
	InvalidModelID uint8 = iota
	ConstantProductID
	StableswapID
)

type Model interface {
	Initialize(reserveX uint64, reserveY uint64, fee uint64, kLast uint64)
	AddLiquidity(amountX uint64, amountY uint64, lpTokenSupply uint64) (uint64, uint64, uint64, error)
	RemoveLiquidity(uint64, uint64) (uint64, uint64, uint64, error)
	// Amount IN, tokenIN
	// AmountOut, error
	Swap(uint64, bool) (uint64, error)
	GetState() (uint64, uint64, uint64)
}

type NewModel func() Model

var Models map[uint8]NewModel

func init() {
	Models = make(map[uint8]NewModel)

	// Append any additional pricing models here
	Models[ConstantProductID] = NewConstantProduct
}
