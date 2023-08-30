// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func TestStorage(t *testing.T) {
	require := require.New(t)

	db := memdb.New()
	pk, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	pub := pk.PublicKey()
	id := uint64(1)

	program := []byte("super cool program")

	err = runtime.SetProgram(db, id, pub, program)
	require.NoError(err)

	ok, owner, prog, err := runtime.GetProgram(db, id)
	require.NoError(err)
	require.True(ok)
	require.Equal(pub[:], owner[:])
	require.Equal(program, prog)
}
