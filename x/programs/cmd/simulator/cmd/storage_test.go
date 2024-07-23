// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrefix(t *testing.T) {
	require := require.New(t)
	stateKey := accountDataKey([]byte{0}, []byte{1, 2, 3})
	require.Equal([]byte{accountPrefix, 0, accountDataPrefix, 1, 2, 3}, stateKey)
	runtime.NewTestProgram()
}
