// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var (
	ErrUnableToConfirmTx = errors.New("unable to confirm transaction")
	ErrInvalidURI        = errors.New("invalid uri")
)

const (
	txCheckInterval = 100 * time.Millisecond
)

type Network struct {
	nodes []*Node
}

func NewNetwork(tc *e2e.GinkgoTestContext) *Network {
	blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
	testNetwork := &Network{}
	for _, uri := range getE2EURIs(tc, blockchainID) {
		n := Node(uri)
		testNetwork.nodes = append(testNetwork.nodes, &n)
	}
	return testNetwork
}

func (n *Network) Nodes() (out []workload.TestNode) {
	for _, node := range n.nodes {
		out = append(out, node)
	}
	return out
}

func (*Network) Configuration() workload.TestNetworkConfiguration {
	return networkConfig
}

type Node string

func (n *Node) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	indexerCli := indexer.NewClient(n.URI())
	for _, tx := range txs {
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, tx.ID())
		if err != nil {
			return err
		}
		if !success {
			return ErrUnableToConfirmTx
		}
	}
	return nil
}

func (n *Node) SubmitTxs(ctx context.Context, txs []*chain.Transaction) error {
	c := jsonrpc.NewJSONRPCClient(n.URI())
	for _, tx := range txs {
		_, err := c.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	c := jsonrpc.NewJSONRPCClient(n.URI())
	_, tx, _, err := c.GenerateTransaction(
		context.Background(),
		networkConfig.Parser(),
		actions,
		auth,
	)
	return tx, err
}

func (n *Node) URI() string {
	return string(*n)
}
