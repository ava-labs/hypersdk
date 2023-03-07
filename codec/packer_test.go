// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/window"
	"github.com/stretchr/testify/require"
)

var (
	TestPublicKey = []byte{
		115, 50, 124, 153, 59, 53, 196, 150, 168, 143, 151, 235,
		222, 128, 136, 161, 9, 40, 139, 85, 182, 153, 68, 135,
		62, 166, 45, 235, 251, 246, 69, 7,
	}
	TestID        = []byte("TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES")
	TestString    = []byte("TestString")
	TestBool      = true
	TestSignature = []byte{2, 8, 143, 126, 80, 159, 186, 93, 157,
		97, 183, 80, 183, 86, 3, 128, 223, 79, 164, 21, 51, 88,
		224, 186, 134, 18, 209, 100, 166, 37, 132, 237, 48, 49,
		102, 144, 53, 111, 245, 209, 141, 252, 154, 0, 111, 229,
		175, 23, 122, 55, 166, 97, 166, 228, 68, 247, 23, 113,
		32, 247, 254, 190, 203, 8}
	TestWindow = []byte{1, 2, 3, 4, 5}
)

func TestNewWriter(t *testing.T) {
	require := require.New(t)
	wr := NewWriter(2)
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

func TestPackerPublicKey(t *testing.T) {
	require := require.New(t)
	wp := NewWriter(crypto.PublicKeyLen)
	var pubKey crypto.PublicKey
	copy(pubKey[:], TestPublicKey)
	// Pack
	wp.PackPublicKey(pubKey)
	require.Equal(TestPublicKey, wp.Bytes(), "PublicKey not packed correctly.")
	require.NoError(wp.Err(), "Error packing PublicKey.")
}

func TestPackerSignature(t *testing.T) {
	require := require.New(t)
	wp := NewWriter(crypto.SignatureLen)
	var sig crypto.Signature
	copy(sig[:], TestSignature)
	// Pack
	wp.PackSignature(sig)
	require.Equal(TestSignature, wp.Bytes())
	require.NoError(wp.Err(), "Error packing Signature.")
}

func TestPackerID(t *testing.T) {
	require := require.New(t)
	// id := ids.GenerateTestID()
	wp := NewWriter(len(ids.ID{}))
	// Pack
	id, _ := ids.ToID(TestID)
	wp.PackID(id)
	// Check packed
	returnedID, err := ids.ToID(wp.Bytes())
	require.NoError(err, "Error retrieving ID.")
	require.Equal(id, returnedID, "ids.ID not packed correctly.")
	require.NoError(wp.Err(), "Error packing ID.")
}

func TestPackerWindow(t *testing.T) {
	require := require.New(t)
	wp := NewWriter(window.WindowSliceSize)
	var wind window.Window
	// Fill window
	copy(wind[:], TestWindow)
	// Pack
	wp.PackWindow(wind)
	// Check packed
	require.Equal(TestWindow, wp.Bytes()[:len(TestWindow)], "Window not packed correctly.")
	require.Equal(window.WindowSliceSize, len(wp.Bytes()), "Window not packed correctly.")
	require.NoError(wp.Err(), "Error packing window.")
}
