// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
)

func OPReaderFromOPWriter(writer *OptionalPacker, size int) *OptionalPacker {
	rp := NewWriter(size)
	rp.PackOptional(writer)
	opr := rp.NewOptionalReader()
	return opr
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
		p := NewWriter(consts.IDLen + 1)
		p.PackByte(1)
		p.PackID(id)
		pr := NewReader(p.Bytes(), consts.IDLen+1)
		opr := pr.NewOptionalReader()

		// fmt.Println(opr.ip.Bytes())
		var unpackedID ids.ID
		// // Unpack
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
