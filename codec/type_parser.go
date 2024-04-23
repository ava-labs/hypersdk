// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"github.com/ava-labs/hypersdk/consts"
)

type decoder[T any, Y any] struct {
	f func(*Packer) (T, error)
	y Y
}

// The number of types is limited to 255.
type TypeParser[T any, Y any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]*decoder[T, Y]
}

// NewTypeParser returns an instance of a Typeparser with generic type [T].
func NewTypeParser[T any, Y bool]() *TypeParser[T, Y] {
	return &TypeParser[T, Y]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]*decoder[T, Y]{},
	}
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *TypeParser[T, Y]) Register(id uint8, f func(*Packer) (T, error), y Y) error {
	if len(p.indexToDecoder) == int(consts.MaxUint8)+1 {
		return ErrTooManyItems
	}
	if _, ok := p.indexToDecoder[id]; ok {
		return ErrDuplicateItem
	}
	p.indexToDecoder[id] = &decoder[T, Y]{f, y}
	return nil
}

// LookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *TypeParser[T, Y]) LookupIndex(index uint8) (func(*Packer) (T, error), bool) {
	d, ok := p.indexToDecoder[index]
	if ok {
		return d.f, true
	}
	return nil, false
}
