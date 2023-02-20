// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/hypersdk/consts"
)

type TypeParser[T any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]func(*Packer) (T, error)

	index uint8
}

func NewTypeParser[T any]() *TypeParser[T] {
	return &TypeParser[T]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]func(*Packer) (T, error){},
	}
}

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

func (p *TypeParser[T]) LookupType(o T) (uint8, func(*Packer) (T, error), bool) {
	index, ok := p.typeToIndex[fmt.Sprintf("%T", o)]
	if !ok {
		return 0, nil, false
	}
	return index, p.indexToDecoder[index], true
}

func (p *TypeParser[T]) LookupIndex(index uint8) (func(*Packer) (T, error), bool) {
	f, ok := p.indexToDecoder[index]
	return f, ok
}
