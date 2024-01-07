// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package types

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
)

type ValKind = wasmtime.ValKind

const (
	TypeI64 ValKind = wasmtime.KindI64
	TypeI32 ValKind = wasmtime.KindI32
)

type Val struct {
	inner wasmtime.Val
}

func (v Val) I32() int32 {
	if v.Kind() != wasmtime.KindI32 {
		panic("not an i32")
	}
	return v.inner.I32()
}

// I64 returns the underlying 64-bit integer if this is an `i64`, or panics.
func (v Val) I64() int64 {
	if v.Kind() != wasmtime.KindI64 {
		panic("not an i64")
	}
	return v.inner.I64()
}

func (v Val) Wasmtime() wasmtime.Val {
	return v.inner
}

func (v Val) Kind() ValKind {
	switch v.inner.Kind() {
	case wasmtime.KindI32:
		return TypeI32
	case wasmtime.KindI64:
		return TypeI64
	default:
		panic("unknown val kind")
	}
}

// ValI32 converts a int32 to a i32 Val
func ValI32(val int32) *Val {
	return &Val{inner: wasmtime.ValI32(val)}
}

// ValI64 converts a go int64 to a i64 Val
func ValI64(val int64) *Val {
	return &Val{inner: wasmtime.ValI64(val)}
}
