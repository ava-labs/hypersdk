// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"strings"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

func TestStorage(t *testing.T) {
	require := require.New(t)

	db := memdb.New()
	pk, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	pub := pk.PublicKey()
	id := uint64(1)

	program := []byte("super cool program")
	f := "test,something,else"
	functionBytes := []byte(f)
	functions := strings.Split(f, ",")

	err = setProgram(db, id, pub, functionBytes, program)
	require.NoError(err)

	ok, owner, fns, prog, err := getProgram(db, id)
	require.NoError(err)
	require.True(ok)
	require.Equal(pub[:], owner[:])
	require.Equal(functions, fns)
	require.Equal(program, prog)
}
