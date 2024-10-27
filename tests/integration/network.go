// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/tests/workload"
)

var (
	ErrUnableToConfirmTx = errors.New("unable to confirm transaction")
	ErrInvalidURI        = errors.New("invalid uri")
	ErrTxNotFound        = errors.New("tx not found")
)

const (
	txCheckInterval = 100 * time.Millisecond
)

type Network struct {
	uris []string
}

func (*Network) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	return instances[0].ConfirmTxs(ctx, txs)
}

func (*Network) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	return instances[0].GenerateTx(ctx, actions, auth)
}

func (i *instance) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	errs := i.vm.Submit(ctx, true, txs)
	if len(errs) != 0 && errs[0] != nil {
		return errs[0]
	}

	expectBlk(i)(false)

	for {
		allFound := true
		for _, tx := range txs {
			err := i.confirmTx(ctx, tx.ID())
			if err == ErrTxNotFound {
				allFound = false
				break
			}
			return err
		}
		if allFound {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.NewTimer(txCheckInterval).C:
			// try again
		}
	}
}

func (i *instance) URI() string {
	return i.routerServer.URL
}

func (i *instance) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	// TODO: support generating tx without using jsonRPC client
	c := jsonrpc.NewJSONRPCClient(i.URI())
	_, tx, _, err := c.GenerateTransaction(
		ctx,
		networkConfig.Parser(),
		actions,
		auth,
	)
	return tx, err
}

func (i *instance) confirmTx(ctx context.Context, txid ids.ID) error {
	lastAcceptedHeight, err := i.vm.GetLastAcceptedHeight()
	if err != nil {
		return err
	}
	lastAcceptedBlockID, err := i.vm.GetBlockHeightID(lastAcceptedHeight)
	if err != nil {
		return err
	}
	blk, err := i.vm.GetBlock(ctx, lastAcceptedBlockID)
	if err != nil {
		return err
	}
	for {
		stflBlk, ok := blk.(*chain.StatefulBlock)
		if !ok {
			return ErrTxNotFound
		}
		for _, tx := range stflBlk.StatelessBlock.Txs {
			if tx.ID() == txid {
				// found.
				return nil
			}
		}
		// keep iterating backward.
		lastAcceptedBlockID = blk.Parent()
		blk, err = i.vm.GetBlock(ctx, lastAcceptedBlockID)
		if err != nil {
			return ErrTxNotFound
		}
	}
}

func (n *Network) URIs() []string {
	return n.uris
}

func (*Network) Configuration() workload.TestNetworkConfiguration {
	return networkConfig
}
