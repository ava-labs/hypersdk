// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var (
	ErrUnableToConfirmTx = errors.New("unable to confirm transaction")
	ErrInvalidURI        = errors.New("invalid uri")

	_ workload.TestNetwork = (*Network)(nil)
)

const (
	txCheckInterval = 100 * time.Millisecond
)

type Network struct {
	network *tmpnet.Network
	// blockchainID is set in the constructor from the network, so that we can determine
	// the correct URI to access the network
	blockchainID          ids.ID
	parser                chain.Parser
	genesisBytes          []byte
	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	ruleFactory           chain.RuleFactory
}

func NewNetwork(tc *e2e.GinkgoTestContext) *Network {
	network := e2e.GetEnv(tc).GetNetwork()
	// load the blockchainID from the network, so that we can determine the correct URI
	// to access the network
	blockchainID := network.GetSubnet(networkConfig.Name()).Chains[0].ChainID
	testNetwork := &Network{
		network:               network,
		blockchainID:          blockchainID,
		parser:                networkConfig.Parser(),
		genesisBytes:          networkConfig.GenesisBytes(),
		genesisAndRuleFactory: networkConfig.GenesisAndRuleFactory(),
	}
	return testNetwork
}

func (n *Network) URIs() []string {
	nodeURIs := n.network.GetNodeURIs()
	uris := make([]string, 0, len(nodeURIs))
	for _, nodeURI := range nodeURIs {
		uris = append(uris, formatURI(nodeURI.URI, n.blockchainID))
	}
	return uris
}

func (n *Network) getRuleFactory(ctx context.Context) (chain.RuleFactory, error) {
	if n.ruleFactory != nil {
		return n.ruleFactory, nil
	}
	uris := n.URIs()
	client := jsonrpc.NewJSONRPCClient(uris[0])

	networkID, _, chainID, err := client.Network(ctx)
	if err != nil {
		return nil, err
	}
	if chainID != n.blockchainID {
		return nil, fmt.Errorf("found unexpected chainID %s != %s", chainID, n.blockchainID)
	}
	_, ruleFactory, err := n.genesisAndRuleFactory.Load(n.genesisBytes, nil, networkID, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to load genesis and rule factory: %w", err)
	}
	n.ruleFactory = ruleFactory
	return ruleFactory, nil
}

func (n *Network) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	uris := n.URIs()
	c := jsonrpc.NewJSONRPCClient(uris[0])
	txIDs := []ids.ID{}
	for _, tx := range txs {
		txID, err := c.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return fmt.Errorf("unable to submit transaction : %w", err)
		}
		txIDs = append(txIDs, txID)
	}

	indexerCli := indexer.NewClient(uris[0])
	for _, txID := range txIDs {
		success, _, err := indexerCli.WaitForTransaction(ctx, txCheckInterval, txID)
		if err != nil {
			return fmt.Errorf("error while waiting for transaction : %w", err)
		}
		if !success {
			return ErrUnableToConfirmTx
		}
	}

	_, targetHeight, _, err := c.Accepted(ctx)
	if err != nil {
		return err
	}
	for _, uri := range uris[1:] {
		if err := jsonrpc.Wait(ctx, txCheckInterval, func(ctx context.Context) (bool, error) {
			c := jsonrpc.NewJSONRPCClient(uri)
			_, nodeHeight, _, err := c.Accepted(ctx)
			if err != nil {
				return false, err
			}
			return nodeHeight >= targetHeight, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (n *Network) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	uris := n.URIs()
	c := jsonrpc.NewJSONRPCClient(uris[0])

	unitPrices, err := c.UnitPrices(ctx, true)
	if err != nil {
		return nil, err
	}
	ruleFactory, err := n.getRuleFactory(ctx)
	if err != nil {
		return nil, err
	}

	return chain.GenerateTransaction(ruleFactory, unitPrices, time.Now().UnixMilli(), actions, auth)
}

func (*Network) Configuration() workload.TestNetworkConfiguration {
	return networkConfig
}
