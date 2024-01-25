// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadKey(t *testing.T) {
	require := require.New(t)

	key1 := NewKey("key1", Read)
	require.True(key1.Permission.HasPermission(Read))
	require.False(key1.Permission.HasPermission(Write))
}

func TestWriteKey(t *testing.T) {
	require := require.New(t)

	key1 := NewKey("key1", Write)
	require.False(key1.Permission.HasPermission(Read))
	require.True(key1.Permission.HasPermission(Write))
}

func TestReadWriteKey(t *testing.T) {
	require := require.New(t)

	key1 := NewKey("key1", Read, Write)
	require.True(key1.Permission.HasPermission(Read))
	require.True(key1.Permission.HasPermission(Write))
}

func TestNoPermissionKey(t *testing.T) {
	require := require.New(t)

	key1 := NewKey("key1")
	require.False(key1.Permission.HasPermission(Read))
	require.False(key1.Permission.HasPermission(Write))
}
