// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state/dbtest"
)

func TestTranslatedImmutableState(t *testing.T) {
	ctx := context.TODO()

	t.Run("value is too short to contain a suffix", func(t *testing.T) {
		r := require.New(t)

		db := dbtest.NewTestDB()
		r.NoError(db.Insert(ctx, testKey, []byte{}))

		translatedState := NewTranslatedImmutable(db)
		v, err := translatedState.GetValue(ctx, testKey)
		r.ErrorIs(err, ErrTooShortForSuffix)
		r.Nil(v)
	})

	t.Run("value not found", func(t *testing.T) {
		r := require.New(t)

		db := dbtest.NewTestDB()
		translatedState := NewTranslatedImmutable(db)

		v, err := translatedState.GetValue(ctx, testKey)
		r.ErrorIs(err, database.ErrNotFound)
		r.Nil(v)
	})

	t.Run("value exists and has suffix", func(t *testing.T) {
		r := require.New(t)

		db := dbtest.NewTestDB()
		r.NoError(db.Insert(
			ctx,
			testKey,
			binary.BigEndian.AppendUint64([]byte("value"), 0)),
		)

		translatedState := NewTranslatedImmutable(db)
		v, err := translatedState.GetValue(ctx, testKey)
		r.NoError(err)
		r.Equal([]byte("value"), v)
	})
}

func FuzzTranslatedMutable(f *testing.F) {
	f.Fuzz(func(t *testing.T, key []byte, value []byte, suffix uint64) {
		ctx := context.TODO()
		r := require.New(t)

		translatedState := NewTranslatedMutable(
			dbtest.NewTestDB(),
			suffix,
		)

		r.NoError(translatedState.Insert(ctx, key, value))

		v, err := translatedState.GetValue(ctx, key)
		r.NoError(err)
		r.Equal(value, v)
	})
}
