// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ava-labs/hypersdk/examples/morpheusvm/tests" // include the tests shared between integration and e2e

	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/tests/registry"
	"github.com/ava-labs/hypersdk/vm/vmtest"

	morpheusvm "github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
)

func TestIntegration(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	vmFactory := morpheusvm.NewFactory()

	testingNetworkConfig, err := workload.NewTestNetworkConfig(0)
	r.NoError(err)

	testNetwork := vmtest.NewTestNetwork(
		ctx,
		t,
		vmFactory,
		2,
		testingNetworkConfig.AuthFactories(),
		testingNetworkConfig.GenesisBytes(),
		nil,
		nil,
	)

	for testRegistry := range registry.GetTestsRegistries() {
		for _, test := range testRegistry.List() {
			t.Run(test.Name, func(t *testing.T) {
				test.Fnc(t, testNetwork)
			})
		}
	}
}
