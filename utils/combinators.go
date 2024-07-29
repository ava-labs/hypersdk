// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

func Map[T any, R any](f func(T) R, a []T) []R {
	b := make([]R, len(a))
	for i, v := range a {
		b[i] = f(v)
	}
	return b
}

func ForEach[T any](f func(T), a []T) {
	for _, v := range a {
		f(v)
	}
}
