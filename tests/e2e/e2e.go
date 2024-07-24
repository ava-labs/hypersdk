// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/tests/workload"
)

var (
	network           workload.Network
	txWorkloadFactory workload.TxWorkloadFactory
)

func SetNetwork(
	n workload.Network,
	factory workload.TxWorkloadFactory,
) {
	network = n
	txWorkloadFactory = factory
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

	ginkgo.It("BasicTxWorkload", func() {
		workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewBasicTxWorkload())
	})

	ginkgo.It("Tx Workloads", func() {
		for _, generator := range txWorkloadFactory.Workloads() {
			workload.ExecuteWorkload(ctx, require, network, generator)
		}
	})

	ginkgo.It("Syncing", ginkgo.Label("Sync"), func() {
		ginkgo.By("Generate 128 blocks", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(128))
		})
		ginkgo.By("Start a new node", func() {}) // TODO
		ginkgo.By("Accept a transaction after state sync", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(1))
		})
		ginkgo.By("Restart the node", func() {}) // TODO
		ginkgo.By("Generate 1024 blocks", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(1024))
		})
		ginkgo.By("Start a new node", func() {}) // TODO
		ginkgo.By("Accept a transaction after state sync", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(1))
		})
		ginkgo.By("Pause the node", func() {}) // TODO
		ginkgo.By("Generate 256 blocks", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(256))
		})
		ginkgo.By("Resume the node", func() {}) // TODO
		ginkgo.By("Accept a transaction after resuming", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(1))
		})
		ginkgo.By("State sync while broadcasting txs", func() {}) // TODO
		ginkgo.By("Accept a transaction after syncing", func() {
			workload.ExecuteWorkload(ctx, require, network, txWorkloadFactory.NewSizedTxWorkload(1))
		})
	})
})
