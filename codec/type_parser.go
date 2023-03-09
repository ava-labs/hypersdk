// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/hypersdk/consts"
)

// The number of types is limited to 255.
type TypeParser[T any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]func(*Packer) (T, error)

	index uint8
}

// NewTypeParser returns an instance of a Typeparser with generic type [T].
func NewTypeParser[T any]() *TypeParser[T] {
	return &TypeParser[T]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]func(*Packer) (T, error){},
	}
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *TypeParser[T]) Register(o T, f func(*Packer) (T, error)) error {
	if p.index == consts.MaxUint8 {
		return ErrTooManyItems
	}
	k := fmt.Sprintf("%T", o)
	if _, ok := p.typeToIndex[k]; ok {
		return ErrDuplicateItem
	}
	p.typeToIndex[k] = p.index
	p.indexToDecoder[p.index] = f
	p.index++
	return nil
}

// LookupType returns the index, decoder function and the success of lookup of
// type [o] from Typeparser [p].
func (p *TypeParser[T]) LookupType(o T) (uint8, func(*Packer) (T, error), bool) {
	index, ok := p.typeToIndex[fmt.Sprintf("%T", o)]
	if !ok {
		return 0, nil, false
	}
	return index, p.indexToDecoder[index], true
}

// LookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *TypeParser[T]) LookupIndex(index uint8) (func(*Packer) (T, error), bool) {
	f, ok := p.indexToDecoder[index]
	return f, ok
}
