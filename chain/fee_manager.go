// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
)

const (
	Bandwidth       Dimension = 0
	Compute         Dimension = 1
	StorageRead     Dimension = 2
	StorageAllocate Dimension = 3
	StorageWrite    Dimension = 4 // includes delete

	FeeDimensions = 5

	DimensionsLen     = consts.Uint64Len * FeeDimensions
	dimensionStateLen = consts.Uint64Len + window.WindowSliceSize + consts.Uint64Len
)

type (
	Dimension  int
	Dimensions [FeeDimensions]uint64
)

func Add(a, b Dimensions) (Dimensions, error) {
	d := Dimensions{}
	for i := Dimension(0); i < FeeDimensions; i++ {
		v, err := math.Add64(a[i], b[i])
		if err != nil {
			return Dimensions{}, err
		}
		d[i] = v
	}
	return d, nil
}

func MulSum(a, b Dimensions) (uint64, error) {
	val := uint64(0)
	for i := Dimension(0); i < FeeDimensions; i++ {
		v, err := math.Mul64(a[i], b[i])
		if err != nil {
			return 0, err
		}
		newVal, err := math.Add64(val, v)
		if err != nil {
			return 0, err
		}
		val = newVal
	}
	return val, nil
}

func (d Dimensions) Add(i Dimension, v uint64) error {
	newValue, err := math.Add64(d[i], v)
	if err != nil {
		return err
	}
	d[i] = newValue
	return nil
}

func (d Dimensions) CanAdd(a Dimensions, l Dimensions) bool {
	for i := Dimension(0); i < FeeDimensions; i++ {
		consumed, err := math.Add64(d[i], a[i])
		if err != nil {
			return false
		}
		if consumed > l[i] {
			return false
		}
	}
	return true
}

func (d Dimensions) Bytes() []byte {
	bytes := make([]byte, DimensionsLen)
	for i := Dimension(0); i < FeeDimensions; i++ {
		binary.BigEndian.PutUint64(bytes[i*consts.Uint64Len:], d[i])
	}
	return bytes
}

// Greater is used to determine if the max units allowed
// are greater than the units consumed by a transaction.
//
// This would be considered a fatal error.
func (d Dimensions) Greater(o Dimensions) bool {
	for i := Dimension(0); i < FeeDimensions; i++ {
		if d[i] < o[i] {
			return false
		}
	}
	return true
}

func UnpackDimensions(raw []byte) (Dimensions, error) {
	if len(raw) != DimensionsLen {
		return Dimensions{}, fmt.Errorf("%w: found=%d wanted=%d", ErrWrongDimensionSize, len(raw), DimensionsLen)
	}
	d := Dimensions{}
	for i := Dimension(0); i < FeeDimensions; i++ {
		d[i] = binary.BigEndian.Uint64(raw[i*consts.Uint64Len:])
	}
	return d, nil
}

func ParseDimensions(raw []string) (Dimensions, error) {
	if len(raw) != FeeDimensions {
		return Dimensions{}, ErrWrongDimensionSize
	}
	d := Dimensions{}
	for i := Dimension(0); i < FeeDimensions; i++ {
		v, err := strconv.ParseUint(raw[i], 10, 64)
		if err != nil {
			return Dimensions{}, err
		}
		d[i] = v
	}
	return d, nil
}
