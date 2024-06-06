// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package collections

import "errors"

type Stack[T any] []T

func (s *Stack[T]) Push(v T) {
	*s = append(*s, v)
}

func (s *Stack[T]) Pop() T {
	res := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return res
}

func (s *Stack[T]) Peek() T {
	return (*s)[len(*s)-1]
}

type FixedSizeStack[T any] struct {
	vals   []T
	length int
}

var ErrMaxStackSize = errors.New("stack limit")

func NewFixedSizeStack[T any](size int) *FixedSizeStack[T] {
	return &FixedSizeStack[T]{vals: make([]T, 0, size), length: size}
}

func (s *FixedSizeStack[T]) Push(v T) error {
	if len(s.vals) >= s.length {
		return ErrMaxStackSize
	}

	s.vals = append(s.vals, v)
	return nil
}

func (s *FixedSizeStack[T]) Pop() T {
	res := s.vals[len(s.vals)-1]
	s.vals = s.vals[:len(s.vals)-1]
	return res
}

func (s *FixedSizeStack[T]) Peek() T {
	return s.vals[len(s.vals)-1]
}
