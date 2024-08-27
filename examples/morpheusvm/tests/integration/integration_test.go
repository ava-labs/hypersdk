// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/tests/integration"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/vm"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	morpheusWorkload "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

func generateConfigs(require *require.Assertions) [][]byte {
	config := vm.NewConfig()
	config.Config = make(map[string]any)
	config.Config["externalSubscriber"] = externalsubscriber.Config{
		Enabled:       true,
		ServerAddress: "localhost:9001",
	}
	cb, err := json.Marshal(config)
	require.NoError(err)

	cbDefault, err := json.Marshal(vm.NewConfig())
	require.NoError(err)

	configBytes := make([][]byte, 3)
	configBytes[0] = cb
	configBytes[1] = cbDefault
	configBytes[2] = cbDefault

	return configBytes
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())
	gen, workloadFactory, err := morpheusWorkload.New(0 /* minBlockGap: 0ms */)
	require.NoError(err)

	parser := controller.NewParser(0, ids.Empty, gen)
	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})
	log, err := logFactory.Make("integrationTest")
	require.NoError(err)

	testSubscriber := workload.NewWorkloadTestSubscriber()
	externalsubscriberServer := externalsubscriber.NewExternalSubscriberServer(
		log,
		controller.ParserFactory,
		[]event.Subscription[externalsubscriber.ExternalSubscriberSubscriptionData]{testSubscriber},
	)
	grpcHandler, err := externalsubscriber.NewGRPCHandler(externalsubscriberServer, log, ":9001")
	require.NoError(err)

	externalSubscriberWorkload := workload.NewExternalSubscriber(*grpcHandler, testSubscriber)
	configs := generateConfigs(require)

	// Setup imports the integration test coverage
	integration.Setup(
		controller.New,
		genesisBytes,
		configs,
		lconsts.ID,
		parser,
		controller.JSONRPCEndpoint,
		workloadFactory,
		randomEd25519AuthFactory,
		externalSubscriberWorkload,
	)
})
