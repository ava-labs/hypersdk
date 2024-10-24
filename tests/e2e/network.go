// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
)

var (
	ErrUnableToConfirmTx = errors.New("unable to confirm transaction")
	ErrInvalidURI        = errors.New("invalid uri")
)

const (
	txCheckInterval = 100 * time.Millisecond
)

type Network struct {
	uris []string
}

func (*Network) ConfirmTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	indexerCli := indexer.NewClient(uri)
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

func (*Network) SubmitTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	c := jsonrpc.NewJSONRPCClient(uri)
	for _, tx := range txs {
		_, err := c.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *Network) URIs() []string {
	return n.uris
}

/*func (*Network) WorkloadFactory() workload.TxWorkloadFactory {
	return txWorkloadFactory
}*/
