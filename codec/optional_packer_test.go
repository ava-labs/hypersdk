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
	return pr.NewOptionalReader()
}

func TestOptionalPackerWriter(t *testing.T) {
	// Initializes empty writer
	require := require.New(t)
	opw := NewOptionalWriter()
	require.Empty(opw.ip.Bytes())
	require.Equal(bits(0), opw.b)
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	// Fill OptionalPacker
	i := 0
	for i <= consts.MaxUint8Offset {
		opw.PackPublicKey(pubKey)
		i += 1
	}
	require.Equal((consts.MaxUint8Offset+1)*crypto.PublicKeyLen, len(opw.ip.Bytes()), "Bytes not added correctly.")
	require.NoError(opw.ip.Err(), "Error packing bytes.")
	opw.PackPublicKey(pubKey)
	require.ErrorIs(ErrTooManyItems, opw.ip.Err(), "Error not thrown after over packing.")
}

func TestOptionalPackerReader(t *testing.T) {
	require := require.New(t)
	// Create packer
	bytes := []byte{1, 0}
	p := NewReader(bytes, 2)
	opr := p.NewOptionalReader()
	require.Equal(bits(1), opr.b)
	require.Equal(bytes, opr.ip.Bytes())
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
	})
}

func TestOptionalPackerPackOptional(t *testing.T) {
	// Packs optional packer correctly
	require := require.New(t)
	p := NewWriter(2)
	op := NewOptionalWriter()
	op.ip.PackByte(byte(90))
	p.PackOptional(op)
	require.Equal([]byte{0, 90}, p.Bytes())
}
