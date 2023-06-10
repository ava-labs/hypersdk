// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/AnomalyFi/hypersdk/consts"
)

type decoder[T any, X any, Y any] struct {
	f func(*Packer, X) (T, error)
	y Y
}

// The number of types is limited to 255.
type TypeParser[T any, X any, Y any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]*decoder[T, X, Y]

	index uint8
}

// NewTypeParser returns an instance of a Typeparser with generic type [T].
func NewTypeParser[T any, X any, Y bool]() *TypeParser[T, X, Y] {
	return &TypeParser[T, X, Y]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]*decoder[T, X, Y]{},
	}
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *TypeParser[T, X, Y]) Register(o T, f func(*Packer, X) (T, error), y Y) error {
	if p.index == consts.MaxUint8 {
		return ErrTooManyItems
	}
	k := fmt.Sprintf("%T", o)
	if _, ok := p.typeToIndex[k]; ok {
		return ErrDuplicateItem
	}
	p.typeToIndex[k] = p.index
	p.indexToDecoder[p.index] = &decoder[T, X, Y]{f, y}
	p.index++
	return nil
}

// LookupType returns the index, decoder function and the success of lookup of
// type [o] from Typeparser [p].
func (p *TypeParser[T, X, Y]) LookupType(o T) (uint8, func(*Packer, X) (T, error), Y, bool) {
	index, ok := p.typeToIndex[fmt.Sprintf("%T", o)]
	if !ok {
		return 0, nil, *new(Y), false
	}
	d := p.indexToDecoder[index]
	return index, d.f, d.y, true
}

// LookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *TypeParser[T, X, Y]) LookupIndex(index uint8) (func(*Packer, X) (T, error), Y, bool) {
	d, ok := p.indexToDecoder[index]
	if ok {
		return d.f, d.y, true
	}
	return nil, *new(Y), false
}
