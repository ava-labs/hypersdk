// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

type TestNetwork interface {
	ConfirmTxs(context.Context, []*chain.Transaction) error
	GenerateTx(context.Context, []chain.Action, chain.AuthFactory) (*chain.Transaction, error)
	URIs() []string
	Configuration() TestNetworkConfiguration
}

// TestNetworkConfiguration is an interface, implemented by the custom-vm network test framework
// that allows the test to store information regarding the test network prior to it's invocation, and
// retrieve the said information during it's execution. It's vital that all implementations of this
// interface would keep the data stored immutable as it would be shared across multiple threads.
type TestNetworkConfiguration interface {
	GenesisBytes() []byte
	Name() string
	Parser() chain.Parser
}
