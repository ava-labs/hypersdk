// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/StephenButtolph/canoto"
)

type Unpacker[T any] interface {
	Unpack(b []byte) (T, error)
}

type Packer[T any] interface {
	Pack(T) ([]byte, error)
}

type CustomCodec[T any] interface {
	Unpacker[T]
	Packer[T]
}

type CustomCodecContext interface {
	Value(key any) interface{}
}

// Embed [CustomCanoto] in place of the original field and this will invoke the custom
// packer/unpacker as needed to handle canoto serialization.

type CustomCanotoInterface[T any] interface {
	*T
}

type CustomCanoto[T any] struct {
	Value T

	bytes []byte
}

func NewCustomCanoto[T any](value T, bytes []byte) CustomCanoto[T] {
	return CustomCanoto[T]{
		Value: value,
		bytes: bytes,
	}
}

func NewMarshalledCustomCanoto[T any](value T, codec CustomCodec[T]) (CustomCanoto[T], error) {
	bytes, err := codec.Pack(value)
	if err != nil {
		return CustomCanoto[T]{}, err
	}
	return NewCustomCanoto[T](value, bytes), nil
}

func NewCustomCanotoUsingCanoto[T canoto.Field](value T) CustomCanoto[T] {
	value.CalculateCanotoCache()
	w := canoto.Writer{B: make([]byte, value.CachedCanotoSize())}
	w = value.MarshalCanotoInto(w)
	return CustomCanoto[T]{
		Value: value,
		bytes: w.B,
	}
}

// Implement [canoto.Field] interface for [CustomCanoto] type and allow arbitrary packer/unpacker
// override.
func (c *CustomCanoto[T]) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	canoto.Append(&w, c.bytes)
	return w
}

func (c *CustomCanoto[T]) CalculateCanotoCache() {}
func (c *CustomCanoto[T]) CachedCanotoSize() int {
	return len(c.bytes)
}
func (c *CustomCanoto[T]) UnmarshalCanotoFrom(r canoto.Reader) error {
	codecContext, ok := r.Context.(CustomCodecContext)
	if !ok {
		return fmt.Errorf("canoto reader context has unexpected type for custom unmarshal: %T", r.Context)
	}
	var emtpyKey T
	codecValue := codecContext.Value(emtpyKey)
	codec, ok := codecValue.(Unpacker[T])
	if !ok {
		return fmt.Errorf("canoto reader context has unexpected value: %T", codecValue)
	}
	res, err := codec.Unpack(r.B)
	if err != nil {
		return err
	}
	c.Value = res
	c.bytes = r.B
	return nil
}

func (c *CustomCanoto[T]) ValidCanoto() bool {
	return len(c.bytes) > 0
}

func (c *CustomCanoto[T]) MakeCanoto() *CustomCanoto[T] {
	return new(CustomCanoto[T])
}
