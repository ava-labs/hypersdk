// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package balance_test

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/state/balance"
)

func TestBalanceHandler(t *testing.T) {
	newBalanceHandler := func() chain.BalanceHandler {
		return balance.NewPrefixBalanceHandler([]byte{0})
	}
	chaintest.TestBalanceHandler(t, context.Background(), newBalanceHandler)
}
