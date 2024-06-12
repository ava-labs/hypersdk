// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"reflect"

	"github.com/near/borsh-go"
)

type customSerialize interface {
	customSerialize() ([]byte, error)
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
	if isNil(value) {
		return nil, nil
	}
	switch t := any(value).(type) {
	case customSerialize:
		return t.customSerialize()
	default:
		return borsh.Serialize(value)
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

func (r RawBytes) customSerialize() ([]byte, error) {
	return r, nil
}

func (r RawBytes) customDeserialize(data []byte) (*RawBytes, error) {
	rawData := RawBytes(data)
	return &rawData, nil
}

type ProgramErrors interface {
	error
}

type DeserializationError struct{}

func (DeserializationError) Error() string {
	return "failed to deserialize"
}

type SerializationError struct{}
type OutOfFuelError struct{}

func (OutOfFuelError) Error() string {
	return "out of fuel"
}

const (
	resultOkPrefix  = byte(1)
	resultErrPrefix = byte(0)
)

type Result[T any, E ProgramErrors] struct {
	hasError bool
	value    T
	e        E
}

func Ok[T any, E ProgramErrors](val T) Result[T, E] {
	return Result[T, E]{value: val}
}

func Err[T any, E ProgramErrors](e E) Result[T, E] {
	return Result[T, E]{e: e, hasError: true}
}

func (r Result[T, E]) isOk() bool {
	return !r.hasError
}

func (r Result[T, E]) isErr() bool {
	return r.hasError
}

func (r Result[T, E]) customSerialize() ([]byte, error) {
	var prefix []byte
	var val any
	if r.isOk() {
		prefix = []byte{resultOkPrefix}
		val = r.value
	} else {
		prefix = []byte{resultErrPrefix}
		val = r.e
	}
	bytes, err := serialize(val)
	if err != nil {
		return nil, err
	}
	return append(prefix, bytes...), nil
}

func (r Result[T, E]) customDeserialize(data []byte) (*Result[T, E], error) {
	if len(data) < 1 {
		return nil, DeserializationError{}
	}
	switch data[0] {
	case resultOkPrefix:
		{
			val, err := deserialize[T](data[1:])
			if err != nil {
				return nil, DeserializationError{}
			}
			return &Result[T, E]{value: *val}, nil
		}
	case resultErrPrefix:
		{
			val, err := deserialize[E](data[1:])
			if err != nil {
				return nil, DeserializationError{}
			}
			return &Result[T, E]{e: *val, hasError: true}, nil
		}
	default:
		return &Result[T, E]{}, DeserializationError{}
	}
}

func (r Result[T, E]) Ok() (T, bool) {
	return r.value, r.isOk()
}

func (r Result[T, E]) Err() (E, bool) {
	return r.e, r.isErr()
}
