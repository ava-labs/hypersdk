// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/vmtest"
)

var _ workload.TestNetwork = (*EVMTestNetwork)(nil)

type EVMTestNetwork struct {
	*vmtest.TestNetwork
}

func NewEVMTestNetwork(ctx context.Context, t *testing.T, factory *vm.Factory, numVMs int, authFactories []chain.AuthFactory, genesisBytes []byte, upgradeBytes []byte, configBytes []byte) *EVMTestNetwork {
	return &EVMTestNetwork{
		TestNetwork: vmtest.NewTestNetwork(
			ctx,
			t,
			factory,
			numVMs,
			authFactories,
			genesisBytes,
			upgradeBytes,
			configBytes,
		),
	}
}
