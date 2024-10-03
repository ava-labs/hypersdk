// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package layout

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec/codectest"
)

func TestLayout(t *testing.T) {
	require := require.New(t)
	addr := codectest.NewRandomAddress()
	defaultLayout := Default()

	// Test that balances are correctly prefixed
	balanceKey := defaultLayout.NewBalanceKey(addr)
	require.True(bytes.HasPrefix(balanceKey, []byte{DefaultBalancePrefix}))

	// Test that an arbitrary key is correctly prefixed
	actionKey := defaultLayout.NewActionKey([]byte("action"), 1)
	require.True(bytes.HasPrefix(actionKey, []byte{DefaultActionPrefix}))
}
