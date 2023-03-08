// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func (o *OptionalPacker) toReader() *OptionalPacker {
	// Add on for bits byte
	size := len(o.ip.Bytes()) + 1
	p := NewWriter(size)
	p.PackOptional(o)
	pr := NewReader(p.Bytes(), size)
	return pr.NewOptionalReader()
}
func TestOptionalPackerWriter(t *testing.T) {
	require := require.New(t)
	require.True(true)
}

func TestOptionalPackerReader(t *testing.T) {
	require := require.New(t)
	require.True(true)
}

func TestOptionalPackerID(t *testing.T) {
	require := require.New(t)
	opw := NewOptionalWriter()
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
		// Pack empty
		opw.PackID(ids.Empty)
		require.Equal(0, len(opw.ip.Bytes()), "PackID packed an empty ID.")
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

func TestOptionalPackerPublicKey(t *testing.T) {
	require := require.New(t)
	require.True(true)
	t.Run("Pack", func(t *testing.T) {

	})
	t.Run("Unpack", func(t *testing.T) {

	})
}

func TestOptionalPackerUint64(t *testing.T) {
	require := require.New(t)
	require.True(true)
	t.Run("Pack", func(t *testing.T) {

	})
	t.Run("Unpack", func(t *testing.T) {

	})
}

func TestOptionalPackerPackOptional(t *testing.T) {
	require := require.New(t)
	require.True(true)
}
