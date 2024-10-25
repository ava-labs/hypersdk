// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
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
	nodes  []*Node
	parser *parser
}

func NewNetwork(tc *e2e.GinkgoTestContext) *Network {
	blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
	testNetwork := &Network{}
	for _, uri := range getE2EURIs(tc, blockchainID) {
		n := &Node{uri: uri, network: testNetwork}
		testNetwork.nodes = append(testNetwork.nodes, n)
	}
	testNetwork.parser = &parser{
		Parser: networkConfig.Parser(),
		rules: &rules{
			Rules:   networkConfig.Parser().Rules(0),
			chainID: blockchainID,
		},
	}
	return testNetwork
}

func (n *Network) URIs() (out []string) {
	for _, node := range n.nodes {
		out = append(out, node.URI())
	}
	return
}

func (n *Network) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	return n.nodes[0].ConfirmTxs(ctx, txs)
}

func (n *Network) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	return n.nodes[0].GenerateTx(ctx, actions, auth)
}

func (*Network) Configuration() workload.TestNetworkConfiguration {
	return networkConfig
}

type Node struct {
	uri     string
	network *Network
}

func (n *Node) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	c := jsonrpc.NewJSONRPCClient(n.URI())
	for _, tx := range txs {
		_, err := c.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return err
		}
	}

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

func (n *Node) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	c := jsonrpc.NewJSONRPCClient(n.URI())
	_, tx, _, err := c.GenerateTransaction(
		ctx,
		n.network.parser,
		actions,
		auth,
	)
	return tx, err
}

func (n *Node) URI() string {
	return n.uri
}

type rules struct {
	chain.Rules
	chainID ids.ID
}

func (r *rules) GetChainID() ids.ID {
	return r.chainID
}

type parser struct {
	chain.Parser
	rules *rules
}

func (p *parser) Rules(int64) chain.Rules {
	return p.rules
}
