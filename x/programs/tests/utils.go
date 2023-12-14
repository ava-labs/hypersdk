// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"os"
	"path/filepath"
	"testing"
)

// ReadFixture reads a file from this fixture directory and returns its content.
func ReadFixture(tb testing.TB, filename string) []byte {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Error(err)
	}
	bytes, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		tb.Error(err)
	}
	return bytes
}
