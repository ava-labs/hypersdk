// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
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

func (n *Network) ConfirmTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	// this is the wrong way. for integration, we wante to test the block content directly.
	instance, err := n.getInstance(uri)
	if err != nil {
		return err
	}
	expectBlk(instance)(false)

	for {
		allFound := true
		for _, tx := range txs {
			err := n.confirmTx(ctx, instance, tx.ID())
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

func (n *Network) SubmitTxs(ctx context.Context, uri string, txs []*chain.Transaction) error {
	instance, err := n.getInstance(uri)
	if err != nil {
		return err
	}
	errs := instance.vm.Submit(ctx, true, txs)
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func (*Network) confirmTx(ctx context.Context, instance instance, txid ids.ID) error {
	lastAcceptedHeight, err := instance.vm.GetLastAcceptedHeight()
	if err != nil {
		return err
	}
	lastAcceptedBlockID, err := instance.vm.GetBlockHeightID(lastAcceptedHeight)
	if err != nil {
		return err
	}
	blk, err := instance.vm.GetBlock(ctx, lastAcceptedBlockID)
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
		blk, err = instance.vm.GetBlock(ctx, lastAcceptedBlockID)
		if err != nil {
			return ErrTxNotFound
		}
	}
}

func (n *Network) URIs() []string {
	return n.uris
}

/*
	func (*Network) WorkloadFactory() workload.TxWorkloadFactory {
		return txWorkloadFactory
	}
*/
func (*Network) getInstance(uri string) (instance, error) {
	for _, instance := range instances {
		if instance.routerServer.URL == uri {
			return instance, nil
		}
	}
	return instance{}, ErrInvalidURI
}
