// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package functions

import (
	"errors"
)

var (
	ErrOutputFunctionDoesNotExist = errors.New("function does not exist")

	ErrReservesZero          = errors.New("reserves are zero")
	ErrBothDeltasZero        = errors.New("both delta values are equal to zero")
	ErrNoClearDeltaToCompute = errors.New("both delta values are nonzero")
)

const (
	InvalidFormulaID = iota // For testing purposes
	ConstantProductFormulaID
)

var _ ConstantFunction = (*ConstantProductFormula)(nil)

var _ ConstantFunction = (*ConstantSumFormula)(nil)

func GetConstantFunction(functionID uint64) (ConstantFunction, error) {
	switch functionID {
	case ConstantProductFormulaID:
		return &ConstantProductFormula{}, nil
	default:
		return nil, ErrOutputFunctionDoesNotExist
	}
}

// All constant functions must implement the following interface
type ConstantFunction interface {
	Swap(reserveX uint64, reserveY uint64, deltaX uint64, deltaY uint64, fee uint64) (uint64, uint64, uint64, uint64, error)
}

type ConstantProductFormula struct{}

// Returns: newReserveX, newReserveY, deltaX, deltaY, error (if exists)
func (*ConstantProductFormula) Swap(reserveX uint64, reserveY uint64, deltaX uint64, deltaY uint64, fee uint64) (uint64, uint64, uint64, uint64, error) {
	if reserveX == 0 || reserveY == 0 {
		return 0, 0, 0, 0, ErrReservesZero
	}
	if deltaX == 0 && deltaY == 0 {
		return 0, 0, 0, 0, ErrBothDeltasZero
	}
	if deltaX != 0 && deltaY != 0 {
		return 0, 0, 0, 0, ErrNoClearDeltaToCompute
	}
	k := reserveX * reserveY
	var output uint64
	if deltaX == 0 {
		// Swapping Y for X
		output = reserveX - (k / (reserveY + ((deltaY * fee) / 100)))
		newReserveX := reserveX - output
		newReserveY := reserveY + deltaY
		return newReserveX, newReserveY, output, deltaY, nil
	} else {
		// Swapping X for Y
		output = reserveY - (k / (reserveX + ((deltaX * fee) / 100)))
		newReserveX := reserveX + deltaX
		newReserveY := reserveY - output
		return newReserveX, newReserveY, deltaX, output, nil
	}
}

type ConstantSumFormula struct{}

func (*ConstantSumFormula) Swap(reserveX uint64, reserveY uint64, deltaX uint64, deltaY uint64, fee uint64) (uint64, uint64, uint64, uint64, error) {
	panic("unimplemented")
}
