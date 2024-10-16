// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/hypersdk/api/indexer"
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
	uris []string
}

func (n *Network) ConfirmTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	// this is the wrong way. for integration, we wante to test the block content directly.
	instance, err := n.getInstance(uri)
	if err != nil {
		return err
	}
	expectBlk(instance)(false)

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

func (n *Network) SubmitTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	instance, err := n.getInstance(uri)
	if err != nil {
		return err
	}
	for _, tx := range txs {
		_, err := instance.cli.SubmitTx(ctx, tx.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *Network) URIs() []string {
	return n.uris
}

func (n *Network) WorkloadFactory() workload.TxWorkloadFactory {
	return txWorkloadFactory
}

func (n *Network) getInstance(uri string) (instance, error) {
	for _, instance := range instances {
		if instance.routerServer.URL == uri {
			return instance, nil
		}
	}
	return instance{}, ErrInvalidURI
}
