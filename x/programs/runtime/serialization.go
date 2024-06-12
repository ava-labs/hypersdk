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
