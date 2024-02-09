// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

// toReader returns an OptionalPacker that is a reader of [o].
// Initializes the OptionalPacker size and bytes from [o]. The returned
// bitmask is equal to the bitmask of [o].
func (o *OptionalPacker) toReader() *OptionalPacker {
	// Add one for o.b byte
	size := len(o.ip.Bytes()) + consts.Uint64Len
	p := NewWriter(10_000, size)
	p.PackOptional(o)
	pr := NewReader(p.Bytes(), size)
	return pr.NewOptionalReader()
}

func TestOptionalPackerID(t *testing.T) {
	opw := NewOptionalWriter(10_000)
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		// Pack empty
		opw.PackID(ids.Empty)
		require.Empty(opw.ip.Bytes(), "PackID packed an empty ID.")

		// Pack ID
		opw.PackID(id)
		bytes, err := ids.ToID(opw.ip.Bytes())
		require.NoError(err, "Error retrieving bytes.")
		require.Equal(id, bytes, "PackID did not set bytes correctly.")
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		opr := opw.toReader()
		var unpackedID ids.ID
		// Unpack
		opr.UnpackID(&unpackedID)
		require.Equal(ids.Empty, unpackedID, "ID unpacked correctly")
		opr.UnpackID(&unpackedID)
		require.Equal(id, unpackedID, "ID unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerUint64(t *testing.T) {
	opw := NewOptionalWriter(10_000)
	val := uint64(900)
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		// Pack empty
		opw.PackUint64(0)
		require.Empty(opw.ip.Bytes(), "PackUint64 packed a zero uint.")

		// Pack ID
		opw.PackUint64(val)
		require.Equal(
			val,
			binary.BigEndian.Uint64(opw.ip.Bytes()),
			"PackUint64 did not set bytes correctly.",
		)
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		opr := opw.toReader()
		require.Equal(uint64(0), opr.UnpackUint64(), "Uint64 unpacked correctly")
		require.Equal(val, opr.UnpackUint64(), "Uint64 unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerAddress(t *testing.T) {
	opw := NewOptionalWriter(10_000)
	id := ids.GenerateTestID()
	addr := CreateAddress(1, id)
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		// Pack empty
		opw.PackAddress(EmptyAddress)
		require.Empty(opw.ip.Bytes(), "PackAddress packed an empty Address.")

		// Pack address
		opw.PackAddress(addr)
		require.True(bytes.Equal(addr[:], opw.ip.Bytes()), "PackPublickey did not set bytes correctly.")
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		opr := opw.toReader()
		var unpackedAddr Address
		opr.UnpackAddress(&unpackedAddr)
		require.True(bytes.Equal(EmptyAddress[:], unpackedAddr[:]), "AddressBytes unpacked correctly")
		opr.UnpackAddress(&unpackedAddr)
		require.Equal(addr, unpackedAddr, "PublicKey unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerInvalidSet(t *testing.T) {
	opw := NewOptionalWriter(10_000)
	val := uint64(900)
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		// Pack empty
		opw.PackUint64(0)
		require.Empty(opw.ip.Bytes(), "PackUint64 packed a zero uint.")

		// Pack ID
		opw.PackUint64(val)
		require.Equal(
			val,
			binary.BigEndian.Uint64(opw.ip.Bytes()),
			"PackUint64 did not set bytes correctly.",
		)
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		// Setup optional reader (expects no entries)
		opr := opw.toReader()
		opr.Done()
		require.ErrorIs(opr.Err(), ErrInvalidBitset)
	})
}
