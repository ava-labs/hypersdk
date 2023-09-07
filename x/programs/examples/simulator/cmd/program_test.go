// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func TestStorage(t *testing.T) {
	require := require.New(t)

	db := memdb.New()

	id := uint64(1)

	program := []byte("super cool program")

	err := runtime.SetProgram(db, id, program)
	require.NoError(err)

	ok, prog, err := runtime.GetProgram(db, id)
	require.NoError(err)
	require.True(ok)
	require.Equal(program, prog)
}
