// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"bytes"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
	"github.com/stretchr/testify/require"
)

var (
	TestString = "TestString"
	TestBool   = true
	TestWindow = []byte{1, 2, 3, 4, 5}
)

func TestNewWriter(t *testing.T) {
	require := require.New(t)
	wr := NewWriter(2, 2)
	require.True(wr.Empty(), "Writer not empty when initialized.")
	// Pack up to the limit
	bytes := []byte{1, 2}
	wr.PackFixedBytes(bytes)
	require.Equal(bytes, wr.Bytes(), "Bytes not packed correctly.")
	// Pack past limit
	wr.PackFixedBytes(bytes)
	require.Equal(2, len(wr.Bytes()), "Bytes overpacked.")
	require.Error(wr.Err(), "Error not set.")
}

func TestPackerID(t *testing.T) {
	require := require.New(t)
	wp := NewWriter(consts.IDLen, consts.IDLen)
	// Pack
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
		wp.PackID(id)
		// Check packed
		returnedID, err := ids.ToID(wp.Bytes())
		require.NoError(err, "Error retrieving ID.")
		require.Equal(id, returnedID, "ids.ID not packed correctly.")
		require.NoError(wp.Err(), "Error packing ID.")
	})
	t.Run("Unpack", func(t *testing.T) {
		// Unpack
		rp := NewReader(wp.Bytes(), consts.IDLen)
		require.Equal(wp.Bytes(), rp.Bytes(), "Reader not initialized correctly.")
		unpackedID := ids.Empty
		rp.UnpackID(true, &unpackedID)
		require.Equal(id, unpackedID, "UnpackID unpacked incorrectly.")
		require.NoError(rp.Err(), "UnpackID set an error.")
		// Unpack again
		unpackedID = ids.Empty
		rp.UnpackID(true, &unpackedID)
		require.Equal(ids.Empty, unpackedID, "UnpackID unpacked incorrectly.")
		require.Error(rp.Err(), "UnpackID did not set error.")
	})
}

func TestPackerWindow(t *testing.T) {
	require := require.New(t)
	wp := NewWriter(window.WindowSliceSize, window.WindowSliceSize)
	var wind window.Window
	// Fill window
	copy(wind[:], TestWindow)
	// Pack
	t.Run("Unpack", func(t *testing.T) {
		wp.PackWindow(wind)
		// Check packed
		require.Equal(TestWindow, wp.Bytes()[:len(TestWindow)], "Window not packed correctly.")
		require.Equal(window.WindowSliceSize, len(wp.Bytes()), "Window not packed correctly.")
		require.NoError(wp.Err(), "Error packing window.")
	})
	t.Run("Unpack", func(t *testing.T) {
		// Unpack
		rp := NewReader(wp.Bytes(), window.WindowSliceSize)
		require.Equal(wp.Bytes(), rp.Bytes(), "Reader not initialized correctly.")
		var unpackedWindow window.Window
		rp.UnpackWindow(&unpackedWindow)
		require.Equal(wind, unpackedWindow, "UnpackWindow unpacked incorrectly.")
		require.NoError(rp.Err(), "UnpackWindow set an error.")
		// Unpack again
		rp.UnpackWindow(&unpackedWindow)
		require.Error(rp.Err(), "UnpackWindow did not set error.")
	})
}

func TestPackerShortBytes(t *testing.T) {
	require := require.New(t)
	t.Run("Pack Too Large", func(t *testing.T) {
		wp := NewWriter(1024, 1024)
		wp.PackShortBytes(make([]byte, 1024))
		require.ErrorIs(wp.Err(), ErrInvalidShortBytes)
	})
	wp := NewWriter(ids.IDLen+1, ids.IDLen+1)
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
		// Pack
		wp.PackShortBytes(id[:])
		b := wp.Bytes()
		require.NoError(wp.Err())
		require.Len(b, ids.IDLen+1)
		require.Equal(uint8(ids.IDLen), b[0])
		require.True(bytes.Equal(id[:], b[1:]))
	})
	t.Run("Unpack", func(t *testing.T) {
		// Unpack
		rp := NewReader(wp.Bytes(), ids.IDLen+1)
		require.Equal(wp.Bytes(), rp.Bytes())
		var unpackedID ShortBytes
		rp.UnpackShortBytes(&unpackedID)
		require.True(bytes.Equal(id[:], unpackedID[:]))
		require.NoError(rp.Err())
	})
}

func TestNewReader(t *testing.T) {
	require := require.New(t)
	vInt := 900
	wp := NewWriter(5, 5)
	// Add an int and a bool
	wp.PackInt(vInt)
	wp.PackBool(true)
	// Create reader
	rp := NewReader(wp.Bytes(), 2)
	require.Equal(wp.Bytes(), rp.Bytes(), "Reader not initialized correctly.")
	// Unpack both values
	require.Equal(vInt, rp.UnpackInt(true), "Reader unpacked correctly.")
	require.True(rp.UnpackBool(), "Reader unpacked correctly.")
	require.NoError(rp.Err(), "Reader set error during unpack.")
	// Unpacked not packed with required
	require.Equal(uint64(0), rp.UnpackUint64(true), "Reader unpacked correctly.")
	require.Error(rp.Err(), "Reader error not set.")
}
