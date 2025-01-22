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

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
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
	uris []string
	// The parser here is the original parser provided by the vm, with the chain ID populated by
	// the newly created network. On e2e networks, we can't tell in advance what the ChainID would be,
	// and therefore need to update it from the network.
	parser *parser
}

func NewNetwork(tc *e2e.GinkgoTestContext) *Network {
	blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
	testNetwork := &Network{
		uris: getE2EURIs(tc, blockchainID),
		parser: &parser{
			Parser: networkConfig.Parser(),
			rules: &rules{
				Rules:   networkConfig.Parser().Rules(0),
				chainID: blockchainID,
			},
		},
	}
	return testNetwork
}

func (n *Network) URIs() []string {
	return n.uris
}

func (n *Network) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	c := jsonrpc.NewJSONRPCClient(n.uris[0])
	txIDs := []ids.ID{}
	for _, tx := range txs {
		txID, err := c.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return fmt.Errorf("unable to submit transaction : %w", err)
		}
		txIDs = append(txIDs, txID)
	}

	indexerCli := indexer.NewClient(n.uris[0])
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
	for _, uri := range n.uris[1:] {
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
	c := jsonrpc.NewJSONRPCClient(n.uris[0])
	_, tx, _, err := c.GenerateTransaction(
		ctx,
		n.parser,
		actions,
		auth,
	)
	return tx, err
}

func (*Network) Configuration() workload.TestNetworkConfiguration {
	return networkConfig
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
