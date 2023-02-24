// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"errors"

	"github.com/ava-labs/avalanchego/utils/buffer"
)

var (
	_ BoundedBuffer[bool] = (*boundedBuffer[bool])(nil)

	errInvalidMaxSize = errors.New("maxSize must be greater than 0")
)

type BoundedBuffer[T any] interface {
	// Insert adds a new value to the buffer. If the buffer is full, the
	// oldest value will be overwritten and [onEvict] will be invoked.
	Insert(elt T)

	// Last retrieves the last item added to the buffer.
	//
	// If no items have been added to the buffer, Last returns the default value of
	// [T] and [false].
	Last() (T, bool)

	// Returns all the items in the buffer sorted from oldest to nest.
	Items() []T
}

// boundedBuffer keeps [maxSize] entries of type [T] in a buffer and calls
// [onEvict] on any item that is overwritten. This is typically used for
// dereferencing old roots during block processing.
//
// boundedBuffer is not thread-safe and requires the caller synchronize usage.
type boundedBuffer[T any] struct {
	innerBuffer buffer.Deque[T]
	maxSize     int
	onEvict     func(T)
}

func NewBoundedBuffer[T any](maxSize int, onEvict func(T)) (BoundedBuffer[T], error) {
	if maxSize < 1 {
		return nil, errInvalidMaxSize
	}
	if onEvict == nil {
		onEvict = func(T) {}
	}
	return &boundedBuffer[T]{
		innerBuffer: buffer.NewUnboundedDeque[T](maxSize + 1), // +1 so we never resize
		maxSize:     maxSize,
		onEvict:     onEvict,
	}, nil
}

func (b *boundedBuffer[T]) Insert(elt T) {
	if b.innerBuffer.Len() == b.maxSize {
		evicted, _ := b.innerBuffer.PopRight()
		b.onEvict(evicted)
	}
	b.innerBuffer.PushRight(elt)
}

func (b *boundedBuffer[T]) Last() (T, bool) {
	return b.innerBuffer.PeekRight()
}

func (b *boundedBuffer[T]) Items() []T {
	return b.innerBuffer.List()
}
