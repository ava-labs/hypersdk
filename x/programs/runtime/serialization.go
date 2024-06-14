// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"bytes"
	"errors"
	"io"
	"reflect"

	"github.com/near/borsh-go"
)

type customSerialize interface {
	customSerialize(b io.Writer) error
}

type customDeserialize[T any] interface {
	customDeserialize([]byte) (*T, error)
}

func deserialize[T any](data []byte) (*T, error) {
	result := new(T)
	var err error
	switch t := any(*result).(type) {
	case customDeserialize[T]:
		return t.customDeserialize(data)
	default:
		err = borsh.Deserialize(result, data)
	}
	return result, err
}

func serialize[T any](value T) ([]byte, error) {
	b := &bytes.Buffer{}
	if err := bufferSerialize(value, b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func bufferSerialize[T any](value T, b io.Writer) error {
	if isNil(value) {
		return nil
	}
	switch t := any(value).(type) {
	case customSerialize:
		return t.customSerialize(b)
	default:
		return borsh.NewEncoder(b).Encode(value)
	}
}

func isNil[T any](t T) bool {
	v := reflect.ValueOf(t)
	kind := v.Kind()
	// Must be one of these types to be nillable
	return (kind == reflect.Ptr ||
		kind == reflect.Interface ||
		kind == reflect.Slice ||
		kind == reflect.Map ||
		kind == reflect.Chan ||
		kind == reflect.Func) &&
		v.IsNil()
}

/// Contains some common types that cross the host/guest boundary

type RawBytes []byte

func (r RawBytes) customSerialize(b io.Writer) error {
	_, err := b.Write(r)
	return err
}

func (RawBytes) customDeserialize(data []byte) (*RawBytes, error) {
	rawData := RawBytes(data)
	return &rawData, nil
}

const (
	resultOkPrefix  = byte(1)
	resultErrPrefix = byte(0)
)

type Result[T any, E any] struct {
	hasError bool
	value    T
	e        E
}

func Ok[T any, E any](val T) Result[T, E] {
	return Result[T, E]{value: val}
}

func Err[T any, E any](e E) Result[T, E] {
	return Result[T, E]{e: e, hasError: true}
}

func (r Result[T, E]) customSerialize(b io.Writer) error {
	var prefix []byte
	var val any
	if r.hasError {
		prefix = []byte{resultErrPrefix}
		val = r.e
	} else {
		prefix = []byte{resultOkPrefix}
		val = r.value
	}
	if _, err := b.Write(prefix); err != nil {
		return err
	}
	return bufferSerialize(val, b)
}

func (Result[T, E]) customDeserialize(data []byte) (*Result[T, E], error) {
	if len(data) < 1 {
		return nil, errors.New("deserialization")
	}
	switch data[0] {
	case resultOkPrefix:
		{
			val, err := deserialize[T](data[1:])
			if err != nil {
				return nil, errors.New("deserialization")
			}
			return &Result[T, E]{value: *val}, nil
		}
	case resultErrPrefix:
		{
			val, err := deserialize[E](data[1:])
			if err != nil {
				return nil, errors.New("deserialization")
			}
			return &Result[T, E]{e: *val, hasError: true}, nil
		}
	default:
		return &Result[T, E]{}, errors.New("deserialization")
	}
}

func (r Result[T, E]) Ok() (T, bool) {
	return r.value, !r.hasError
}

func (r Result[T, E]) Err() (E, bool) {
	return r.e, r.hasError
}

const (
	optionSomePrefix = byte(1)
	optionNonePrefix = byte(0)
)

type Option[T any] struct {
	isNone bool
	value  T
}

func Some[T any](val T) Option[T] {
	return Option[T]{value: val}
}

func None[T any]() Option[T] {
	return Option[T]{isNone: true}
}

func (o Option[T]) customSerialize(b io.Writer) error {
	if o.isNone {
		_, err := b.Write([]byte{optionNonePrefix})
		return err
	}
	if _, err := b.Write([]byte{optionSomePrefix}); err != nil {
		return err
	}
	return bufferSerialize(o.value, b)
}

func (Option[T]) customDeserialize(data []byte) (*Option[T], error) {
	if len(data) < 1 {
		return nil, errors.New("deserialization")
	}
	switch data[0] {
	case optionSomePrefix:
		{
			val, err := deserialize[T](data[1:])
			if err != nil {
				return nil, errors.New("deserialization")
			}
			return &Option[T]{value: *val}, nil
		}
	case optionNonePrefix:
		{
			return &Option[T]{isNone: true}, nil
		}
	default:
		return &Option[T]{isNone: true}, errors.New("deserialization")
	}
}

func (o Option[T]) Some() (T, bool) {
	return o.value, !o.isNone
}

func (o Option[T]) None() bool {
	return o.isNone
}
