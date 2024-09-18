// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

type Blah interface {
	Typed
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

type withID struct {
	ID uint8
}

func (w *withID) GetTypeID() uint8 { return w.ID }
func TestTypeParser(t *testing.T) {
	tp := NewTypeParser[Blah]()

	t.Run("empty parser", func(t *testing.T) {
		require := require.New(t)
		f, ok := tp.lookupIndex(0)
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
			tp.Register(blah1, func(*Packer) (Blah, error) { return nil, errBlah1 }),
		)
		require.NoError(
			tp.Register(blah2, func(*Packer) (Blah, error) { return nil, errBlah2 }),
		)

		f, ok := tp.lookupIndex(blah1.GetTypeID())
		require.True(ok)
		res, err := f(nil)
		require.Nil(res)
		require.ErrorIs(err, errBlah1)

		f, ok = tp.lookupIndex(blah2.GetTypeID())
		require.True(ok)
		res, err = f(nil)
		require.Nil(res)
		require.ErrorIs(err, errBlah2)
	})

	t.Run("duplicate item", func(t *testing.T) {
		require := require.New(t)
		err := tp.Register(&Blah1{}, nil)
		require.ErrorIs(err, ErrDuplicateItem)
	})

	t.Run("too many items", func(t *testing.T) {
		require := require.New(t)
		arrayLength := int(consts.MaxUint8) + 1 - len(tp.indexToDecoder)
		for index := range make([]struct{}, arrayLength) {
			// 0 and 1 are already existing -> we use index + 2
			require.NoError(tp.Register(&withID{ID: uint8(index + 2)}, nil))
		}
		// all possible uint8 value should already be stored, using any return ErrTooManyItems
		err := tp.Register(&withID{ID: uint8(4)}, nil)
		require.ErrorIs(err, ErrTooManyItems)
	})
}
