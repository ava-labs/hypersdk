// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/stretchr/testify/require"
)

// toReader returns an OptionalPacker that is a reader of [o].
// Initializes the OptionalPacker size and bytes from [o]. The returned
// bitmask is equal to the bitmask of [o].
func (o *OptionalPacker) toReader() *OptionalPacker {
	// Add one for o.b byte
	size := len(o.ip.Bytes()) + 1
	p := NewWriter(size)
	p.PackOptional(o)
	pr := NewReader(p.Bytes(), size)
	return pr.NewOptionalReader(8)
}

func TestOptionalPackerWriter(t *testing.T) {
	// Initializes empty writer with a limit two byte limit
	require := require.New(t)
	opw := NewOptionalWriter(consts.MaxUint8Offset + 1)
	require.Empty(opw.ip.Bytes())
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	// Fill OptionalPacker
	i := 0
	for i <= consts.MaxUint8Offset {
		opw.PackPublicKey(pubKey)
		i += 1
	}
	require.Equal(
		(consts.MaxUint8Offset+1)*crypto.PublicKeyLen,
		len(opw.ip.Bytes()),
		"Bytes not added correctly.",
	)
	require.NoError(opw.ip.Err(), "Error packing bytes.")
	opw.PackPublicKey(pubKey)
	require.ErrorIs(opw.ip.Err(), ErrTooManyItems, "Error not thrown after over packing.")
}

func TestOptionalPackerTwoByteLimit(t *testing.T) {
	// Initializes empty writer and have o.b exceed one byte of storage.
	require := require.New(t)
	opw := NewOptionalWriter(32)
	require.Empty(opw.ip.Bytes())
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	// Fill OptionalPacker, bits exceeds one byte
	i := 0
	for i <= 31 {
		opw.PackPublicKey(pubKey)
		i += 1
	}
	require.Equal(32*crypto.PublicKeyLen, len(opw.ip.Bytes()), "Bytes not added correctly.")
	require.NoError(opw.ip.Err(), "Error packing bytes.")
	// Check PackOptional
	p := NewWriter(len(opw.ip.Bytes()) + len(opw.b.Bytes()))
	p.PackOptional(opw)
	require.NoError(p.Err(), "Error packing OptionalPacker.")
	require.Equal(
		len(opw.ip.Bytes())+len(opw.b.Bytes()),
		len(p.Bytes()),
		"Error packing OptionalPacker.",
	)

	// opw.PackPublicKey(pubKey)
	// require.ErrorIs(opw.ip.Err(), ErrTooManyItems, "Error not thrown after over packing.")
}

func TestOptionalPackerReader(t *testing.T) {
	require := require.New(t)
	// Create packer
	bytes := []byte{1, 0}
	p := NewReader(bytes, 2)
	opr := p.NewOptionalReader(8)
	require.Equal(1, opr.b.BitLen())
	require.Equal(bytes, opr.ip.Bytes())
}

func TestOptionalPackerPublicKey(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter(MaxItems)
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
	})
}

func TestOptionalPackerID(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter(MaxItems)
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
	})
}

func TestOptionalPackerUint64(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter(MaxItems)
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
	})
}

func TestOptionalPackerPackOptional(t *testing.T) {
	// Packs optional packer correctly
	require := require.New(t)
	p := NewWriter(2)
	op := NewOptionalWriter(MaxItems)
	op.ip.PackByte(byte(90))
	p.PackOptional(op)
	require.Equal([]byte{0, 90}, p.Bytes())
}
