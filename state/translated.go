// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	_ Immutable = (*TranslatedImmutable)(nil)
	_ Mutable   = (*TranslatedMutable)(nil)

	ErrValueTooShortForSuffix = errors.New("value too short for suffix")
)

type TranslatedImmutable struct {
	im Immutable
}

func NewTranslatedImmutable(im Immutable) TranslatedImmutable {
	return TranslatedImmutable{im: im}
}

// GetValue reads from a suffix-based key-value store.
func (t TranslatedImmutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return innerGetValue(t.im.GetValue(ctx, key))
}

type TranslatedMutable struct {
	mu     Mutable
	suffix uint64
}

func NewTranslatedMutable(mu Mutable, suffix uint64) TranslatedMutable {
	return TranslatedMutable{
		mu:     mu,
		suffix: suffix,
	}
}

// GetValue reads from a suffix-based key-value store.
// The value is returned with the suffix removed
func (t TranslatedMutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return innerGetValue(t.mu.GetValue(ctx, key))
}

// GetValue writes to a suffix-based key-value store.
// The suffix associated with t is appended to the value before writing.
func (t TranslatedMutable) Insert(ctx context.Context, key []byte, value []byte) error {
	value = binary.BigEndian.AppendUint64(value, t.suffix)
	return t.mu.Insert(ctx, key, value)
}

func (t TranslatedMutable) Remove(ctx context.Context, key []byte) error {
	return t.mu.Remove(ctx, key)
}

func innerGetValue(v []byte, err error) ([]byte, error) {
	if err == database.ErrNotFound {
		return v, err
	}
	if err != nil {
		return nil, err
	}
	if len(v) < consts.Uint64Len {
		return nil, ErrValueTooShortForSuffix
	}
	return v[:len(v)-consts.Uint64Len], nil
}
