// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "morpheusvm-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single MorpheusVM Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	gen, workloadFactory, err := workload.New(100 /* minBlockGap: 100ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes())
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject MorpheusVM name
	// and workload factory to orchestrate the test.
	he2e.SetWorkload(consts.Name, workloadFactory, expectedABI)

	tc := e2e.NewTestContext()

	return fixture.NewTestEnvironment(tc, flagVars, owner, consts.Name, consts.ID, genesisBytes).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})

var _ = ginkgo.Describe("[MorpheusVM APIs]", func() {
	tc := e2e.NewTestContext()
	require := require.New(tc)

	ginkgo.It("Ping", func() {
		client := jsonrpc.NewJSONRPCClient("http://127.0.0.1:9650/ext/bc/" + consts.Name)
		ok, err := client.Ping(tc.DefaultContext())
		require.NoError(err)
		require.True(ok)
	})

	ginkgo.It("Calls Transfer action via JSONRPC", func() {
		client := jsonrpc.NewJSONRPCClient("http://127.0.0.1:9650/ext/bc/" + consts.Name)

		senderAddr, err := codec.ParseAddressBech32(consts.HRP, "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu")
		require.NoError(err)

		receiverAddr, err := codec.ParseAddressBech32(consts.HRP, "morpheus1qrar5gu3mx0syrkxesd66yzk0h8ep7sy8dejzu4crgkjkcynnj8k6qh5wjj")
		require.NoError(err)

		transfer := &actions.Transfer{
			To:    receiverAddr,
			Value: 1,
			Memo:  []byte("test"),
		}
		transferResults, errorString, err := client.ExecuteAction(tc.DefaultContext(), transfer, 0, senderAddr)
		require.NoError(err)
		require.Equal("", errorString)
		require.Len(transferResults, 1)

		transferResultBytes, err := json.Marshal(transferResults[0])
		require.NoError(err)

		var transferResult actions.TransferResult
		err = json.Unmarshal(transferResultBytes, &transferResult)
		require.NoError(err)

		require.Equal(actions.TransferResult{
			SenderBalance:   10000000000000,
			ReceiverBalance: 1,
		}, transferResult)
	})
})
