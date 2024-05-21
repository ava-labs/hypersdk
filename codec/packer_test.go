// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
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
	require.Len(wr.Bytes(), 2, "Bytes overpacked.")
	require.ErrorIs(wr.Err(), wrappers.ErrInsufficientLength)
}

func TestPackerID(t *testing.T) {
	wp := NewWriter(consts.IDLen, consts.IDLen)
	id := ids.GenerateTestID()
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		wp.PackID(id)
		returnedID, err := ids.ToID(wp.Bytes())
		require.NoError(err, "Error retrieving ID.")
		require.Equal(id, returnedID, "ids.ID not packed correctly.")
		require.NoError(wp.Err(), "Error packing ID.")
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		rp := NewReader(wp.Bytes(), consts.IDLen)
		require.Equal(wp.Bytes(), rp.Bytes(), "Reader not initialized correctly.")
		unpackedID := ids.Empty
		rp.UnpackID(true, &unpackedID)
		require.Equal(id, unpackedID, "UnpackID unpacked incorrectly.")
		require.NoError(rp.Err(), "UnpackID set an error.")

		// Unpacking again should error
		unpackedID = ids.Empty
		rp.UnpackID(true, &unpackedID)
		require.Equal(ids.Empty, unpackedID, "UnpackID unpacked incorrectly.")
		require.ErrorIs(rp.Err(), wrappers.ErrInsufficientLength)
	})
}

func TestPackerWindow(t *testing.T) {
	wp := NewWriter(window.WindowSliceSize, window.WindowSliceSize)
	var wind window.Window
	// Fill window
	copy(wind[:], TestWindow)
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		wp.PackWindow(wind)
		require.Equal(TestWindow, wp.Bytes()[:len(TestWindow)], "Window not packed correctly.")
		require.Len(wp.Bytes(), window.WindowSliceSize, "Window not packed correctly.")
		require.NoError(wp.Err(), "Error packing window.")
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		rp := NewReader(wp.Bytes(), window.WindowSliceSize)
		require.Equal(wp.Bytes(), rp.Bytes(), "Reader not initialized correctly.")
		var unpackedWindow window.Window
		rp.UnpackWindow(&unpackedWindow)
		require.Equal(wind, unpackedWindow, "UnpackWindow unpacked incorrectly.")
		require.NoError(rp.Err(), "UnpackWindow set an error.")
		// Unpacking again should error
		rp.UnpackWindow(&unpackedWindow)
		require.ErrorIs(rp.Err(), wrappers.ErrInsufficientLength)
	})
}

func TestPackerAddress(t *testing.T) {
	wp := NewWriter(AddressLen, AddressLen)
	id := ids.GenerateTestID()
	addr := CreateAddress(1, id)
	t.Run("Pack", func(t *testing.T) {
		require := require.New(t)

		wp.PackAddress(addr)
		b := wp.Bytes()
		require.NoError(wp.Err())
		require.Len(b, AddressLen)
		require.Equal(uint8(1), b[0])
		require.Equal(id[:], b[1:])
	})
	t.Run("Unpack", func(t *testing.T) {
		require := require.New(t)

		rp := NewReader(wp.Bytes(), AddressLen)
		require.Equal(wp.Bytes(), rp.Bytes())
		var unpackedAddr Address
		rp.UnpackAddress(&unpackedAddr)
		require.Equal(addr[:], unpackedAddr[:])
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
	require.Zero(rp.UnpackUint64(true), "Reader unpacked correctly.")
	require.ErrorIs(rp.Err(), wrappers.ErrInsufficientLength)
}
