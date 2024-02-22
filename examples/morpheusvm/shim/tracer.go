// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

var _ state.Mutable = (*StateKeyTracer)(nil)

type StateKeyTracer struct {
	state.Mutable
	Keys state.Keys
}

func NewStateKeyTracer(m state.Mutable) *StateKeyTracer {
	return &StateKeyTracer{
		Mutable: m,
		Keys:    make(state.Keys),
	}
}

func (s *StateKeyTracer) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	s.Keys.Add(string(key), state.Read)
	return s.Mutable.GetValue(ctx, key)
}

func (s *StateKeyTracer) Insert(ctx context.Context, key, value []byte) error {
	s.Keys.Add(string(key), state.Write) // TODO: Handle Allocate permission
	return s.Mutable.Insert(ctx, key, value)
}

func (s *StateKeyTracer) Remove(ctx context.Context, key []byte) error {
	s.Keys.Add(string(key), state.Write)
	return s.Mutable.Remove(ctx, key)
}
