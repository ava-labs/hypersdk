// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"fmt"
	"reflect"

	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/hypersdk/consts"
)

type decoder[T Typed] struct {
	f func([]byte) (T, error)
}

type Typed interface {
	GetTypeID() uint8
}

type CanotoTyped interface {
	Typed
	CanotoSpec(...reflect.Type) *canoto.Spec
}

type TypedStruct struct {
	Name string `canoto:"string,1" json:"name"`
	ID   uint8  `canoto:"uint,2"   json:"id"`

	canotoData canotoData_TypedStruct
}

type CanotoParser[T Typed] struct {
	registeredTypes []*canoto.Spec
	indexToDecoder  map[uint8]*decoder[T]
	typeToIndex     map[string]uint8
}

func NewCanotoParser[T Typed]() *CanotoParser[T] {
	return &CanotoParser[T]{
		registeredTypes: []*canoto.Spec{},
		indexToDecoder:  map[uint8]*decoder[T]{},
		typeToIndex:     map[string]uint8{},
	}
}

// Register registers a new type into TypeParser [p]. Registers the type by using
// the string representation of [o], and sets the decoder of that index to [f].
// Returns an error if [o] has already been registered or the TypeParser is full.
func (p *CanotoParser[T]) Register(instance CanotoTyped, f func([]byte) (T, error)) error {
	if len(p.indexToDecoder) == int(consts.MaxUint8)+1 {
		return ErrTooManyItems
	}

	if _, ok := p.indexToDecoder[instance.GetTypeID()]; ok {
		return ErrDuplicateItem
	}
	p.indexToDecoder[instance.GetTypeID()] = &decoder[T]{f: f}

	spec := instance.CanotoSpec()
	p.registeredTypes = append(p.registeredTypes, spec)
	p.typeToIndex[spec.Name] = instance.GetTypeID()

	return nil
}

// Unmarshal unmarshals a value of type [T] from the reader by unpacking
// the typeID and invoking the corresponding decoder function.
func (p *CanotoParser[T]) Unmarshal(bytes []byte) (T, error) {
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

func (p *CanotoParser[T]) GetRegisteredTypes() []*canoto.Spec {
	return p.registeredTypes
}

func (p *CanotoParser[T]) GetTypedStructs() []TypedStruct {
	typedStructs := make([]TypedStruct, 0)
	for k, v := range p.typeToIndex {
		typedStructs = append(typedStructs, TypedStruct{
			Name: k,
			ID:   v,
		})
	}
	return typedStructs
}

// lookupIndex returns the decoder function and success of lookup of [index]
// from Typeparser [p].
func (p *CanotoParser[T]) lookupIndex(index uint8) (func([]byte) (T, error), bool) {
	d, ok := p.indexToDecoder[index]
	if ok {
		return d.f, true
	}
	return nil, false
}
