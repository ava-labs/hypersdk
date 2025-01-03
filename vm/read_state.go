// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ state.Immutable = (*RState)(nil)

	errKeyNotFetched = errors.New("key not fetched")
)

type RStateValue struct {
	value []byte
	err   error
}

type RState struct {
	mp map[string]RStateValue
}

func (r *RState) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	v, ok := r.mp[string(key)]
	if !ok {
		return nil, errKeyNotFetched
	}
	return v.value, v.err
}

func NewRState(keys [][]byte, values [][]byte, errs []error) *RState {
	mp := make(map[string]RStateValue, len(keys))
	for i, key := range keys {
		mp[string(key)] = RStateValue{
			value: values[i],
			err:   errs[i],
		}
	}
	return &RState{mp: mp}
}
