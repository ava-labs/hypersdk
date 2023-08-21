// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package math

import "github.com/ava-labs/avalanchego/utils/math"

type Uint64Operator struct {
	v   uint64
	err error
}

func NewUint64Operator(v uint64) *Uint64Operator {
	return &Uint64Operator{
		v: v,
	}
}

func (o *Uint64Operator) Add(n uint64) {
	if o.err != nil {
		return
	}

	nv, err := math.Add64(o.v, n)
	if err != nil {
		o.err = err
		return
	}
	o.v = nv
}

func (o *Uint64Operator) Mul(n uint64) {
	if o.err != nil {
		return
	}

	nv, err := math.Mul64(o.v, n)
	if err != nil {
		o.err = err
		return
	}
	o.v = nv
}

func (o *Uint64Operator) MulAdd(a, b uint64) {
	if o.err != nil {
		return
	}

	pv, err := math.Mul64(a, b)
	if err != nil {
		o.err = err
		return
	}
	nv, err := math.Add64(o.v, pv)
	if err != nil {
		o.err = err
		return
	}
	o.v = nv
}

func (o *Uint64Operator) Value() (uint64, error) {
	return o.v, o.err
}
