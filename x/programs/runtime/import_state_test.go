// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"
)

func TestImportStatePutGet(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "state_access")

	result, err := program.Call("put", int64(10))
	require.NoError(err)
	require.Nil(result)

	result, err = program.Call("get")
	require.NoError(err)
	valueBytes, err := serialize(int64(10))
	require.NoError(err)
	require.Equal(append([]byte{1}, valueBytes...), result)
}

func TestImportStateRemove(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "state_access")

	valueBytes, err := borsh.Serialize(int64(10))
	require.NoError(err)

	result, err := program.Call("put", int64(10))
	require.NoError(err)
	require.Nil(result)

	result, err = program.Call("delete")
	require.NoError(err)
	require.Equal(append([]byte{1}, valueBytes...), result)

	result, err = program.Call("get")
	require.NoError(err)
	require.Equal([]byte{0}, result)
}

func TestImportStateDeleteMissingKey(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "state_access")

	result, err := program.Call("delete")
	require.NoError(err)
	require.Equal([]byte{0}, result)
}

func TestImportStateGetMissingKey(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "state_access")

	result, err := program.Call("get")
	require.NoError(err)
	require.Equal([]byte{0}, result)
}
