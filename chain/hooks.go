// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "github.com/ava-labs/hypersdk/state/tstate"

var _ Hooks = (*DefaultHooks)(nil)

type Hooks interface {
	Before(storage map[string][]byte, height uint64) (map[string][]byte, error)
	After(ts *tstate.TState) error
}

type DefaultHooks struct{}

func (d DefaultHooks) Before(storage map[string][]byte, height uint64) (map[string][]byte, error) {
	return storage, nil
}

func (d DefaultHooks) After(ts *tstate.TState) error {
	return nil
}
