// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pricing

import (
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ Model = (*ConstantProduct)(nil)

type ConstantProduct struct {
	reserveX uint64
	reserveY uint64
	fee      uint64
	kLast    uint64
}

func NewConstantProduct(
	reserveX uint64,
	reserveY uint64,
	fee uint64,
	kLast uint64,
) Model {
	return &ConstantProduct{
		reserveX: reserveX,
		reserveY: reserveY,
		fee:      fee,
		kLast:    kLast,
	}
}

// Returns: tokens to actor, tokens to owner, tokens to "burn"
func (c *ConstantProduct) AddLiquidity(amountX uint64, amountY uint64, lpTokenSupply uint64) (uint64, uint64, uint64, error) {
	var (
		liquidity     uint64
		tokensToOwner uint64
		tokensToBurn  uint64
	)
	tokensToOwner, err := c.computeOwnerFees(lpTokenSupply)
	if err != nil {
		return 0, 0, 0, err
	}
	if lpTokenSupply == 0 {
		newK, err := smath.Mul(amountX, amountY)
		if err != nil {
			return 0, 0, 0, err
		}
		liquidity = sqrt(newK)
		liquidity, err = smath.Sub(liquidity, storage.MinimumLiquidity)
		if err != nil {
			return 0, 0, 0, err
		}
		tokensToBurn = storage.MinimumLiquidity
	} else {
		tokenXChange, err := smath.Mul(amountX, lpTokenSupply)
		if err != nil {
			return 0, 0, 0, err
		}
		tokenXChange /= c.reserveX
		tokenYChange, err := smath.Mul(amountY, lpTokenSupply)
		if err != nil {
			return 0, 0, 0, err
		}
		tokenYChange /= c.reserveY
		liquidity = min(tokenXChange, tokenYChange)
	}

	if liquidity == 0 {
		return 0, 0, 0, ErrOutputInsufficientLiquidityMinted
	}

	c.reserveX += amountX
	c.reserveY += amountY
	kLast, err := smath.Mul(c.reserveX, c.reserveY)
	if err != nil {
		return 0, 0, 0, err
	}
	c.kLast = kLast

	return liquidity, tokensToOwner, tokensToBurn, nil
}

// Inputs: tokensToBurn, lpTotalSupply
// Returns: owner fees, output X, output Y, error
func (c *ConstantProduct) RemoveLiquidity(tokensToBurn uint64, lpTotalSupply uint64) (uint64, uint64, uint64, error) {
	// Compute owner fees
	// Note: lpTotalSupply ~= liquidity
	tokensToOwner, err := c.computeOwnerFees(lpTotalSupply)
	if err != nil {
		return 0, 0, 0, err
	}
	outputX, err := smath.Mul(c.reserveX, tokensToBurn)
	if err != nil {
		return 0, 0, 0, err
	}
	outputX /= lpTotalSupply
	outputY, err := smath.Mul(c.reserveY, tokensToBurn)
	if err != nil {
		return 0, 0, 0, err
	}
	outputY /= lpTotalSupply

	c.reserveX -= outputX
	c.reserveY -= outputY
	c.kLast, err = smath.Mul(c.reserveX, c.reserveY)
	if err != nil {
		return 0, 0, 0, err
	}
	return outputX, outputY, tokensToOwner, nil
}

// Returns: outputX, outputY, error
func (c *ConstantProduct) Swap(amountX uint64, amountY uint64) (uint64, uint64, error) {
	if c.reserveX == 0 || c.reserveY == 0 {
		return 0, 0, ErrReservesZero
	}
	if amountX == 0 && amountY == 0 {
		return 0, 0, ErrBothDeltasZero
	}
	if amountX != 0 && amountY != 0 {
		return 0, 0, ErrNoClearDeltaToCompute
	}
	k := c.reserveX * c.reserveY
	var output uint64
	if amountX == 0 {
		// Swapping Y for X
		output = c.reserveX - ((1000 * k) / ((1000 * c.reserveY) + (amountY * c.fee)))
		c.reserveX -= output
		c.reserveY += amountY
		return output, amountY, nil
	} else {
		// Swapping X for Y
		output = c.reserveY - ((1000 * k) / ((1000 * c.reserveX) + (amountX * c.fee)))
		c.reserveX += amountX
		c.reserveY -= output
		return amountX, output, nil
	}
}

func (c *ConstantProduct) GetState() (uint64, uint64, uint64) {
	return c.reserveX, c.reserveY, c.kLast
}

// Returns: tokens to mint to owner
func (c *ConstantProduct) computeOwnerFees(lpTotalSupply uint64) (uint64, error) {
	k, err := smath.Mul(c.reserveX, c.reserveY)
	if err != nil {
		return 0, err
	}
	rootK := sqrt(k)
	rootKLast := sqrt(c.kLast)
	if rootK > rootKLast {
		num, err := smath.Sub(rootK, rootKLast)
		if err != nil {
			return 0, err
		}
		num, err = smath.Mul(lpTotalSupply, num)
		if err != nil {
			return 0, err
		}
		denom, err := smath.Mul(rootK, 5)
		if err != nil {
			return 0, err
		}
		denom, err = smath.Add(denom, rootKLast)
		if err != nil {
			return 0, err
		}
		liquidity := num / denom
		if liquidity > 0 {
			return liquidity, nil
		}
	}
	return 0, nil
}

// https://github.com/Uniswap/v2-core/blob/ee547b17853e71ed4e0101ccfd52e70d5acded58/contracts/libraries/Math.sol#L10
func sqrt(y uint64) uint64 {
	if y > 3 {
		z := y
		x := (y / 2) + 1
		for x < z {
			z = x
			x = (y/x + x) / 2
		}
		return z
	} else if y != 0 {
		return 1
	}
	return 0
}

func min(x uint64, y uint64) uint64 {
	if x < y {
		return x
	} else {
		return y
	}
}
