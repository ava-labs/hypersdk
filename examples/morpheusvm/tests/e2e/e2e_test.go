// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/e2e"
	"github.com/ava-labs/hypersdk/tests/workload"

	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	// TODO
	// These vars are set via CLI flags from the run script to support the e2e tests and running
	// a local MorpheusVM network.
	// Decouple this to run a standard e2e test suite and a separate CLI tool to launch a local network.
	requestTimeout time.Duration

	networkRunnerLogLevel      string
	avalanchegoLogLevel        string
	avalanchegoLogDisplayLevel string
	gRPCEp                     string
	gRPCGatewayEp              string

	execPath  string
	pluginDir string

	vmGenesisPath    string
	vmConfigPath     string
	subnetConfigPath string
	outputPath       string

	mode string

	numValidators uint

	_ workload.TxWorkloadFactory  = (*workloadFactory)(nil)
	_ workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
)

func init() {
	flag.DurationVar(
		&requestTimeout,
		"request-timeout",
		120*time.Second,
		"timeout for transaction issuance and confirmation",
	)

	flag.StringVar(
		&networkRunnerLogLevel,
		"network-runner-log-level",
		"info",
		"gRPC server endpoint",
	)

	flag.StringVar(
		&avalanchegoLogLevel,
		"avalanchego-log-level",
		"info",
		"log level for avalanchego",
	)

	flag.StringVar(
		&avalanchegoLogDisplayLevel,
		"avalanchego-log-display-level",
		"error",
		"log display level for avalanchego",
	)

	flag.StringVar(
		&gRPCEp,
		"network-runner-grpc-endpoint",
		"0.0.0.0:8080",
		"gRPC server endpoint",
	)
	flag.StringVar(
		&gRPCGatewayEp,
		"network-runner-grpc-gateway-endpoint",
		"0.0.0.0:8081",
		"gRPC gateway endpoint",
	)

	flag.StringVar(
		&execPath,
		"avalanchego-path",
		"",
		"avalanchego executable path",
	)

	flag.StringVar(
		&pluginDir,
		"avalanchego-plugin-dir",
		"",
		"avalanchego plugin directory",
	)

	flag.StringVar(
		&vmGenesisPath,
		"vm-genesis-path",
		"",
		"VM genesis file path",
	)

	flag.StringVar(
		&vmConfigPath,
		"vm-config-path",
		"",
		"VM configfile path",
	)

	flag.StringVar(
		&subnetConfigPath,
		"subnet-config-path",
		"",
		"Subnet configfile path",
	)

	flag.StringVar(
		&outputPath,
		"output-path",
		"",
		"output YAML path to write local cluster information",
	)

	flag.StringVar(
		&mode,
		"mode",
		"test",
		"'test' to shut down cluster after tests, 'run' to skip tests and only run without shutdown",
	)

	flag.UintVar(
		&numValidators,
		"num-validators",
		5,
		"number of validators per blockchain",
	)
}

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	// Load default pk
	// Address: morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu
	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	require.NoError(err)
	priv := ed25519.PrivateKey(privBytes)
	factory := auth.NewED25519Factory(priv)

	// Read the VM config in, so it can be passed in as JSON.
	// ANR sometimes takes the param as a file path or JSON string
	// and other times only a JSON string.
	vmConfigData, err := os.ReadFile(vmConfigPath)
	require.NoError(err)
	vmConfig := string(vmConfigData)

	e2e.CreateE2ENetwork(
		context.Background(),
		e2e.Config{
			Name:                       consts.Name,
			Mode:                       mode,
			ExecPath:                   execPath,
			NumValidators:              numValidators,
			GRPCEp:                     gRPCEp,
			NetworkRunnerLogLevel:      networkRunnerLogLevel,
			AvalanchegoLogLevel:        avalanchegoLogLevel,
			AvalanchegoLogDisplayLevel: avalanchegoLogDisplayLevel,
			VMGenesisPath:              vmGenesisPath,
			VMConfig:                   vmConfig,
			SubnetConfigPath:           subnetConfigPath,
			PluginDir:                  pluginDir,
		},
	)

	e2e.SetWorkloadFactory(
		&workloadFactory{factory: factory},
	)
})

type workloadFactory struct {
	factory *auth.ED25519Factory
}

func (f *workloadFactory) NewBasicTxWorkload(uri string) (workload.TxWorkloadIterator, error) {
	return f.NewSizedTxWorkload(uri, 1)
}

func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
	cli := rpc.NewJSONRPCClient(uri)
	networkID, _, blockchainID, err := cli.Network(context.Background())
	if err != nil {
		return nil, err
	}
	lcli := lrpc.NewJSONRPCClient(uri, networkID, blockchainID)
	return &simpleTxWorkload{
		factory: f.factory,
		cli:     cli,
		lcli:    lcli,
		size:    size,
	}, nil
}

type simpleTxWorkload struct {
	factory   *auth.ED25519Factory
	cli       *rpc.JSONRPCClient
	lcli      *lrpc.JSONRPCClient
	networkID uint32
	chainID   ids.ID
	count     int
	size      int
}

func (g *simpleTxWorkload) Next() bool {
	return g.count < g.size
}

func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, func(ctx context.Context, uri string) error, error) {
	g.count++
	other, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return nil, nil, err
	}

	aother := auth.NewED25519Address(other.PublicKey())
	aotherStr := codec.MustAddressBech32(consts.HRP, aother)
	parser, err := g.lcli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	}
	_, tx, _, err := g.cli.GenerateTransaction(
		ctx,
		parser,
		[]chain.Action{&actions.Transfer{
			To:    aother,
			Value: 1,
		}},
		g.factory,
	)
	if err != nil {
		return nil, nil, err
	}

	return tx, func(ctx context.Context, uri string) error {
		lcli := lrpc.NewJSONRPCClient(uri, g.networkID, g.chainID)
		success, _, err := lcli.WaitForTransaction(ctx, tx.ID())
		if err != nil {
			return fmt.Errorf("failed to wait for tx %s: %w", tx.ID(), err)
		}
		if !success {
			return fmt.Errorf("tx %s not accepted", tx.ID())
		}
		balance, err := lcli.Balance(ctx, aotherStr)
		if err != nil {
			return fmt.Errorf("failed to get balance of %s: %w", aotherStr, err)
		}
		if balance != 1 {
			return fmt.Errorf("expected balance of 1, got %d", balance)
		}
		return nil
	}, nil
}
