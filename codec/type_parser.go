// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"fmt"

	"github.com/ava-labs/hypersdk/consts"
)

type decoder[T Typed] struct {
	f func([]byte) (T, error)
}

// The number of types is limited to 255.
type TypeParser[T Typed] struct {
	typeToIndex     map[string]uint8
	indexToDecoder  map[uint8]*decoder[T]
	registeredTypes []Typed
}

// NewTypeParser returns an instance of a Typeparser with generic type [T].
func NewTypeParser[T Typed]() *TypeParser[T] {
	return &TypeParser[T]{
		typeToIndex:     map[string]uint8{},
		indexToDecoder:  map[uint8]*decoder[T]{},
		registeredTypes: []Typed{},
	}
}

type Typed interface {
	GetTypeID() uint8
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *TypeParser[T]) Register(instance Typed, f func([]byte) (T, error)) error {
	if len(p.indexToDecoder) == int(consts.MaxUint8)+1 {
		return ErrTooManyItems
	}

	if _, ok := p.indexToDecoder[instance.GetTypeID()]; ok {
		return ErrDuplicateItem
	}
	p.indexToDecoder[instance.GetTypeID()] = &decoder[T]{f: f}
	p.registeredTypes = append(p.registeredTypes, instance)

	return nil
}

// lookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *TypeParser[T]) lookupIndex(index uint8) (func([]byte) (T, error), bool) {
	d, ok := p.indexToDecoder[index]
	if ok {
		return d.f, true
	}
	return nil, false
}

// Unmarshal unmarshals a value of type [T] from the reader by unpacking
// the typeID and invoking the corresponding decoder function.
func (p *TypeParser[T]) Unmarshal(bytes []byte) (T, error) {
	if len(bytes) == 0 {
		return *new(T), fmt.Errorf("typeID not found in slice with length %d", len(bytes))
	}

	typeID := bytes[0]
	decoder, ok := p.lookupIndex(typeID)
	if !ok {
		return *new(T), fmt.Errorf("typeID %d not found", typeID)
	}

	// Note: we include the typeID in the slice to be unpacked.
	// This ensures that for a given type T, Unmarshal(Bytes(T)) == T
	// This is preferred over two alternatives:
	// 1. A separate component prepends the typeID to the result of Bytes(T)
	// 2. Bytes(T) includes the typeID, and Unmarshal assumes the typeID has been stripped
	return decoder(bytes)
}

// GetRegisteredTypes returns all registered types in the TypeParser.
// This is used for generating ABI.
func (p *TypeParser[T]) GetRegisteredTypes() []Typed {
	return p.registeredTypes
}
