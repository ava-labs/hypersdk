// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/hypersdk/consts"
)

type TypeParser[T any, X any] struct {
	typeToIndex    map[string]uint8
	indexToDecoder map[uint8]func(*Packer, X) (T, error)

	index uint8
}

func NewTypeParser[T any, X any]() *TypeParser[T, X] {
	return &TypeParser[T, X]{
		typeToIndex:    map[string]uint8{},
		indexToDecoder: map[uint8]func(*Packer, X) (T, error){},
	}
}

func (p *TypeParser[T, X]) Register(o T, f func(*Packer, X) (T, error)) error {
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

func (p *TypeParser[T, X]) LookupType(o T) (uint8, func(*Packer, X) (T, error), bool) {
	index, ok := p.typeToIndex[fmt.Sprintf("%T", o)]
	if !ok {
		return 0, nil, false
	}
	return index, p.indexToDecoder[index], true
}

func (p *TypeParser[T, X]) LookupIndex(index uint8) (func(*Packer, X) (T, error), bool) {
	f, ok := p.indexToDecoder[index]
	return f, ok
}
