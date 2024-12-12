// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scope

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ Scope = (*DefaultScope)(nil)
	_ Scope = (*SimulatedScope)(nil)
	_ Scope = (*TieredScope)(nil)
)

var ErrValueTooShortToBeSuffixed = errors.New("value is too short to be suffixed")

type Scope interface {
	Has(key []byte, perm state.Permissions) bool
	GetValue(ctx context.Context, key []byte) ([]byte, error)
	Len() int
	Refund(RefundRules) (fees.Dimensions, error)
}

type TieredScope struct {
	keys        state.Keys
	storage     map[string][]byte
	epsilon     uint64
	blockHeight uint64

	hotKeys map[string]uint16
}

func (d *TieredScope) GetValue(_ context.Context, key []byte) ([]byte, error) {
	if v, has := d.storage[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func (d *TieredScope) Has(key []byte, perm state.Permissions) bool {
	return d.keys[string(key)].Has(perm)
}

func (d *TieredScope) Len() int {
	return len(d.keys)
}

func NewTieredScope(
	ks state.Keys,
	storage map[string][]byte,
	epsilon uint64,
	blockHeight uint64,
) (*TieredScope, error) {
	st := make(map[string][]byte, len(storage))
	hotKeys := make(map[string]uint16)

	for k, v := range storage {
		if len(v) < consts.Uint64Len {
			return nil, ErrValueTooShortToBeSuffixed
		}

		lastTouched := binary.BigEndian.Uint64(v[len(v)-consts.Uint64Len:])

		// If hot key
		if blockHeight-lastTouched <= epsilon {
			maxChunks, ok := keys.MaxChunks([]byte(k))
			if !ok {
				return nil, fmt.Errorf("failed to parse max chunks for key %s", k)
			}
			hotKeys[k] = maxChunks
		}

		v = v[:len(v)-consts.Uint64Len]
		st[k] = v
	}

	return &TieredScope{
		keys:        ks,
		storage:     st,
		epsilon:     epsilon,
		blockHeight: blockHeight,
		hotKeys:     hotKeys,
	}, nil
}

func (d *TieredScope) Refund(r RefundRules) (fees.Dimensions, error) {
	readsRefundOp := math.NewUint64Operator(0)
	for _, v := range d.hotKeys {
		readsRefundOp.Add(r.GetStorageKeyReadRefundUnits())
		readsRefundOp.MulAdd(uint64(v), r.GetStorageValueReadRefundUnits())
	}
	readRefunds, err := readsRefundOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	return fees.Dimensions{0, 0, readRefunds, 0, 0}, nil
}

type DefaultScope struct {
	keys    state.Keys
	storage map[string][]byte
}

func NewDefaultScope(
	keys state.Keys,
	storage map[string][]byte,
) *DefaultScope {
	return &DefaultScope{
		keys:    keys,
		storage: storage,
	}
}

func (d *DefaultScope) GetValue(_ context.Context, key []byte) ([]byte, error) {
	if v, has := d.storage[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func (d *DefaultScope) Has(key []byte, perm state.Permissions) bool {
	return d.keys[string(key)].Has(perm)
}

func (d *DefaultScope) Len() int {
	return len(d.keys)
}

func (*DefaultScope) Refund(_ RefundRules) (fees.Dimensions, error) {
	return fees.Dimensions{}, nil
}

// TODO: is this even scope?
type SimulatedScope struct {
	keys state.Keys
	im   state.Immutable
}

func NewSimulatedScope(keys state.Keys, im state.Immutable) *SimulatedScope {
	return &SimulatedScope{
		keys: keys,
		im:   im,
	}
}

func (*SimulatedScope) Refund(_ RefundRules) (fees.Dimensions, error) {
	return fees.Dimensions{}, nil
}

func (d *SimulatedScope) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	return d.im.GetValue(ctx, key)
}

func (d *SimulatedScope) Has(key []byte, perm state.Permissions) bool {
	d.keys.Add(string(key), perm)
	return true
}

func (d *SimulatedScope) Len() int {
	return len(d.keys)
}

func (d *SimulatedScope) StateKeys() state.Keys {
	return d.keys
}

// Flush clears the keys in the scope
func (d *SimulatedScope) Flush() {
	clear(d.keys)
}
