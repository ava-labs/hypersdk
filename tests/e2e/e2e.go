// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/tests/workload"
)

var network workload.Network

func SetNetwork(n workload.Network) {
	network = n
}

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	ctx := context.Background()
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("Ping", func() {
		workload.Ping(ctx, require, network)
	})

	ginkgo.It("GetNetwork", func() {
		workload.GetNetwork(ctx, require, network)
	})
})
