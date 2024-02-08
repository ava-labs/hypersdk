// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestSaveBytes(t *testing.T) {
	require := require.New(t)

	tempDir := os.TempDir()
	filename := filepath.Join(tempDir, "SaveBytes")

	id := ids.GenerateTestID()
	require.NoError(SaveBytes(filename, id[:]), "Error during call to SaveBytes")
	require.FileExists(filename, "SaveBytes did not create file")

	// Check correct key was saved in file
	bytes, err := os.ReadFile(filename)
	var lid ids.ID
	copy(lid[:], bytes)
	require.NoError(err, "Reading saved file threw an error")
	require.Equal(id, lid, "ID is different than saved key")

	// Remove File
	_ = os.Remove(filename)
}

func TestLoadBytesIncorrectLength(t *testing.T) {
	// Creates dummy file with invalid size
	require := require.New(t)
	invalidBytes := []byte{1, 2, 3, 4, 5}

	// Writes
	f, err := os.CreateTemp("", "TestLoadBytes*")
	require.NoError(err)
	fileName := f.Name()
	err = os.WriteFile(fileName, invalidBytes, 0o600)
	require.NoError(err, "Error writing using OS during tests")
	err = f.Close()
	require.NoError(err, "Error closing file during tests")

	// Validate
	_, err = LoadBytes(fileName, ids.IDLen)
	require.ErrorIs(err, ErrInvalidSize)

	// Remove file
	_ = os.Remove(fileName)
}

func TestLoadKeyInvalidFile(t *testing.T) {
	require := require.New(t)

	filename := "FileNameDoesntExist"
	_, err := LoadBytes(filename, ids.IDLen)
	require.ErrorIs(err, os.ErrNotExist)
}

func TestLoadBytes(t *testing.T) {
	require := require.New(t)

	// Creates dummy file with valid size
	f, err := os.CreateTemp("", "TestLoadKey*")
	require.NoError(err)
	fileName := f.Name()
	id := ids.GenerateTestID()
	_, err = f.Write(id[:])
	require.NoError(err)
	err = f.Close()
	require.NoError(err)

	// Validate
	lid, err := LoadBytes(fileName, ids.IDLen)
	require.NoError(err)
	require.True(bytes.Equal(lid, id[:]))

	// Remove
	_ = os.Remove(fileName)
}
