// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

type Blah interface {
	Bark() string
}
type Blah1 struct{}

func (*Blah1) Bark() string { return "blah1" }

func (*Blah1) GetTypeID() uint8 { return 0 }

type Blah2 struct{}

func (*Blah2) Bark() string { return "blah2" }

func (*Blah2) GetTypeID() uint8 { return 1 }

type Blah3 struct{}

func (*Blah3) Bark() string { return "blah3" }

func (*Blah3) GetTypeID() uint8 { return 2 }

func TestTypeParser(t *testing.T) {
	tp := NewTypeParser[Blah, bool]()

	t.Run("empty parser", func(t *testing.T) {
		require := require.New(t)
		f, ok := tp.LookupIndex(0)
		require.Nil(f)
		require.False(ok)
	})

	t.Run("populated parser", func(t *testing.T) {
		require := require.New(t)

		blah1 := &Blah1{}
		blah2 := &Blah2{}
		errBlah1 := errors.New("blah1")
		errBlah2 := errors.New("blah2")
		require.NoError(
			tp.Register(
				blah1.GetTypeID(),
				func(*Packer) (Blah, error) { return nil, errBlah1 },
				true,
			),
		)
		require.NoError(
			tp.Register(
				blah2.GetTypeID(),
				func(*Packer) (Blah, error) { return nil, errBlah2 },
				false,
			),
		)

		f, ok := tp.LookupIndex(blah1.GetTypeID())
		require.True(ok)
		res, err := f(nil)
		require.Nil(res)
		require.ErrorIs(err, errBlah1)

		f, ok = tp.LookupIndex(blah2.GetTypeID())
		require.True(ok)
		res, err = f(nil)
		require.Nil(res)
		require.ErrorIs(err, errBlah2)
	})

	t.Run("duplicate item", func(t *testing.T) {
		require := require.New(t)
		require.ErrorIs(tp.Register((&Blah1{}).GetTypeID(), nil, true), ErrDuplicateItem)
	})

	t.Run("too many items", func(t *testing.T) {
		require := require.New(t)
		arrayLength := int(consts.MaxUint8) + 1 - len(tp.indexToDecoder)
		for index := range make([]struct{}, arrayLength) {
			// 0 and 1 are already existing -> we use index + 2
			require.NoError(tp.Register(uint8(index+2), nil, true))
		}
		// all possible uint8 value should already be store, using any return ErrTooManyItems
		require.ErrorIs(tp.Register(uint8(4), nil, true), ErrTooManyItems)
	})
}
