// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"errors"
	"testing"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
)

type Blah interface {
	Bark() string
}
type Blah1 struct{}

func (*Blah1) Bark() string { return "blah1" }

type Blah2 struct{}

func (*Blah2) Bark() string { return "blah2" }

type Blah3 struct{}

func (*Blah3) Bark() string { return "blah3" }

func TestTypeParser(t *testing.T) {
	tp := NewTypeParser[Blah]()

	t.Run("empty parser", func(t *testing.T) {
		require := require.New(t)
		f, ok := tp.LookupIndex(0)
		require.Nil(f)
		require.False(ok)
		index, f, ok := tp.LookupType(&Blah1{})
		require.Equal(uint8(0), index)
		require.Nil(f)
		require.False(ok)
	})

	t.Run("populated parser", func(t *testing.T) {
		require := require.New(t)
		require.NoError(
			tp.Register(
				&Blah1{},
				func(p *Packer) (Blah, error) { return nil, errors.New("blah1") },
			),
		)
		require.Equal(uint8(1), tp.index)
		require.NoError(
			tp.Register(
				&Blah2{},
				func(p *Packer) (Blah, error) { return nil, errors.New("blah2") },
			),
		)
		require.Equal(uint8(2), tp.index)

		f, ok := tp.LookupIndex(0)
		require.True(ok)
		res, err := f(nil)
		require.Nil(res)
		require.ErrorContains(err, "blah1")

		index, f, ok := tp.LookupType(&Blah1{})
		require.True(ok)
		require.Equal(uint8(0), index)
		res, err = f(nil)
		require.Nil(res)
		require.ErrorContains(err, "blah1")

		f, ok = tp.LookupIndex(1)
		require.True(ok)
		res, err = f(nil)
		require.Nil(res)
		require.ErrorContains(err, "blah2")

		index, f, ok = tp.LookupType(&Blah2{})
		require.True(ok)
		require.Equal(uint8(1), index)
		res, err = f(nil)
		require.Nil(res)
		require.ErrorContains(err, "blah2")
	})

	t.Run("duplicate item", func(t *testing.T) {
		require := require.New(t)
		require.ErrorIs(tp.Register(&Blah1{}, nil), ErrDuplicateItem)
	})

	t.Run("too many items", func(t *testing.T) {
		require := require.New(t)
		tp.index = consts.MaxUint8 // force max
		require.ErrorIs(tp.Register(&Blah3{}, nil), ErrTooManyItems)
	})
}
