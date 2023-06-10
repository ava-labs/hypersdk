// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto"
	"github.com/stretchr/testify/require"
)

// toReader returns an OptionalPacker that is a reader of [o].
// Initializes the OptionalPacker size and bytes from [o]. The returned
// bitmask is equal to the bitmask of [o].
func (o *OptionalPacker) toReader() *OptionalPacker {
	// Add one for o.b byte
	size := len(o.ip.Bytes()) + consts.Uint64Len
	p := NewWriter(size)
	p.PackOptional(o)
	pr := NewReader(p.Bytes(), size)
	return pr.NewOptionalReader()
}

func TestOptionalPackerWriter(t *testing.T) {
	// Initializes empty writer with a limit two byte limit
	require := require.New(t)
	opw := NewOptionalWriter()
	require.Empty(opw.ip.Bytes())
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	// Fill OptionalPacker
	i := 0
	for i <= consts.MaxUint64Offset {
		opw.PackPublicKey(pubKey)
		i += 1
	}
	require.Equal(
		(consts.MaxUint64Offset+1)*crypto.PublicKeyLen,
		len(opw.ip.Bytes()),
		"Bytes not added correctly.",
	)
	require.NoError(opw.Err(), "Error packing bytes.")
	opw.PackPublicKey(pubKey)
	require.ErrorIs(opw.Err(), ErrTooManyItems, "Error not thrown after over packing.")
}

func TestOptionalPackerPublicKey(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter()
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	t.Run("Pack", func(t *testing.T) {
		// Pack empty
		opw.PackPublicKey(crypto.EmptyPublicKey)
		require.Empty(opw.ip.Bytes(), "PackPublickey packed an empty ID.")
		// Pack ID
		opw.PackPublicKey(pubKey)
		require.Equal(TestPublicKey, opw.ip.Bytes(), "PackPublickey did not set bytes correctly.")
	})
	t.Run("Unpack", func(t *testing.T) {
		// Setup optional reader
		opr := opw.toReader()
		var unpackedPubkey crypto.PublicKey
		// Unpack
		opr.UnpackPublicKey(&unpackedPubkey)
		require.Equal(crypto.EmptyPublicKey[:], unpackedPubkey[:], "PublicKey unpacked correctly")
		opr.UnpackPublicKey(&unpackedPubkey)
		require.Equal(pubKey, unpackedPubkey, "PublicKey unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerID(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter()
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
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
		// Setup optional reader
		opr := opw.toReader()
		var unpackedID ids.ID
		// // Unpack
		opr.UnpackID(&unpackedID)
		require.Equal(ids.Empty, unpackedID, "ID unpacked correctly")
		opr.UnpackID(&unpackedID)
		require.Equal(id, unpackedID, "ID unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerUint64(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter()
	val := uint64(900)
	t.Run("Pack", func(t *testing.T) {
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
		// Setup optional reader
		opr := opw.toReader()
		// Unpack
		require.Equal(uint64(0), opr.UnpackUint64(), "Uint64 unpacked correctly")
		require.Equal(val, opr.UnpackUint64(), "Uint64 unpacked correctly")
		opr.Done()
		require.NoError(opr.Err())
	})
}

func TestOptionalPackerInvalidSet(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter()
	val := uint64(900)
	t.Run("Pack", func(t *testing.T) {
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
		// Setup optional reader (expects no entries)
		opr := opw.toReader()
		opr.Done()
		require.ErrorIs(opr.Err(), ErrInvalidBitset)
	})
}
