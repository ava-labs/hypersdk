// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ReadFixture reads a file from this fixture directory and returns its content.
func ReadFixture(tb testing.TB, filename string) []byte {
	require := require.New(tb)
	tb.Helper()
	dir, err := os.Getwd()
	require.NoError(err)
	bytes, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(err)
	return bytes
}
