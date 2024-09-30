// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("state_access")
	require.NoError(err)

	result, err := contract.Call("put", int64(10))
	require.NoError(err)
	require.Nil(result)

	result, err = contract.Call("get")
	require.NoError(err)
	valueBytes, err := Serialize(int64(10))
	require.NoError(err)
	require.Equal(Some[RawBytes](valueBytes), into[Option[RawBytes]](result))
}

func TestImportStateRemove(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("state_access")
	require.NoError(err)

	valueBytes, err := borsh.Serialize(int64(10))
	require.NoError(err)

	result, err := contract.Call("put", int64(10))
	require.NoError(err)
	require.Nil(result)

	result, err = contract.Call("delete")
	require.NoError(err)
	require.Equal(Some[RawBytes](valueBytes), into[Option[RawBytes]](result))

	result, err = contract.Call("get")
	require.NoError(err)
	require.Equal(None[RawBytes](), into[Option[RawBytes]](result))
}

func TestImportStateDeleteMissingKey(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("state_access")
	require.NoError(err)

	result, err := contract.Call("delete")
	require.NoError(err)
	require.Equal(None[RawBytes](), into[Option[RawBytes]](result))
}

func TestImportStateGetMissingKey(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("state_access")
	require.NoError(err)

	result, err := contract.Call("get")
	require.NoError(err)
	require.Equal(None[RawBytes](), into[Option[RawBytes]](result))
}
