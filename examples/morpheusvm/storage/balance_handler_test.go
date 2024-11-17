// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

func NewBalanceHandler() chain.BalanceHandler {
	return &BalanceHandler{}
}

func TestBalanceHandler(t *testing.T) {
	chaintest.TestBalanceHandler(t, context.Background(), NewBalanceHandler)
}
