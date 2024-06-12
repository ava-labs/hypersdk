// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"reflect"

	"github.com/near/borsh-go"
)

type RawBytes []byte

func deserialize[T any](data []byte) (*T, error) {
	result := new(T)
	var err error
	switch t := any(result).(type) {
	case *RawBytes:
		*t = data
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
	case RawBytes:
		return t, nil
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

type ProgramErrors interface {
}

type resultType uint8

const (
	resultOk  resultType = 1
	resultErr resultType = 0
)

type wrappedVal[T any] struct {
	val T
}
type Result[T any, E ProgramErrors] struct {
	resultType resultType `borsh_enum:"true"`
	e          wrappedVal[E]
	v          wrappedVal[T]
}

func Ok[T any, E ProgramErrors](val T) Result[T, E] {
	return Result[T, E]{resultType: resultOk, v: wrappedVal[T]{val: val}}
}

func Err[T any, E ProgramErrors](e E) Result[T, E] {
	return Result[T, E]{resultType: resultErr, e: wrappedVal[E]{val: e}}
}

func (r Result[T, E]) isOk() bool {
	return r.resultType == resultOk
}

func (r Result[T, E]) isErr() bool {
	return r.resultType == resultErr
}

func (r Result[T, E]) Ok() (T, bool) {
	return r.v.val, r.isOk()
}

func (r Result[T, E]) Err() (E, bool) {
	return r.e.val, r.isErr()
}
