// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"errors"
	"testing"

	"github.com/AnomalyFi/hypersdk/consts"
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
	tp := NewTypeParser[Blah, any, bool]()

	t.Run("empty parser", func(t *testing.T) {
		require := require.New(t)
		f, b, ok := tp.LookupIndex(0)
		require.Nil(f)
		require.False(ok)
		require.False(b)
		index, f, b, ok := tp.LookupType(&Blah1{})
		require.Equal(uint8(0), index)
		require.Nil(f)
		require.False(ok)
		require.False(b)
	})

	t.Run("populated parser", func(t *testing.T) {
		require := require.New(t)
		require.NoError(
			tp.Register(
				&Blah1{},
				func(p *Packer, a any) (Blah, error) { return nil, errors.New("blah1") },
				true,
			),
		)
		require.Equal(uint8(1), tp.index)
		require.NoError(
			tp.Register(
				&Blah2{},
				func(p *Packer, a any) (Blah, error) { return nil, errors.New("blah2") },
				false,
			),
		)
		require.Equal(uint8(2), tp.index)

		f, b, ok := tp.LookupIndex(0)
		require.True(ok)
		require.True(b)
		res, err := f(nil, nil)
		require.Nil(res)
		require.ErrorContains(err, "blah1")

		index, f, b, ok := tp.LookupType(&Blah1{})
		require.True(ok)
		require.True(b)
		require.Equal(uint8(0), index)
		res, err = f(nil, nil)
		require.Nil(res)
		require.ErrorContains(err, "blah1")

		f, b, ok = tp.LookupIndex(1)
		require.True(ok)
		require.False(b)
		res, err = f(nil, nil)
		require.Nil(res)
		require.ErrorContains(err, "blah2")

		index, f, b, ok = tp.LookupType(&Blah2{})
		require.True(ok)
		require.False(b)
		require.Equal(uint8(1), index)
		res, err = f(nil, nil)
		require.Nil(res)
		require.ErrorContains(err, "blah2")
	})

	t.Run("duplicate item", func(t *testing.T) {
		require := require.New(t)
		require.ErrorIs(tp.Register(&Blah1{}, nil, true), ErrDuplicateItem)
	})

	t.Run("too many items", func(t *testing.T) {
		require := require.New(t)
		tp.index = consts.MaxUint8 // force max
		require.ErrorIs(tp.Register(&Blah3{}, nil, true), ErrTooManyItems)
	})
}
