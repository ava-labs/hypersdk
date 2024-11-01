// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"errors"
	"fmt"

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

type Network struct {
	uris []string
}

func (*Network) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	err := instances[0].confirmTxs(ctx, txs)
	if err != nil {
		return err
	}
	lastAcceptedBlock := instances[0].vm.LastAcceptedBlock()
	for i := 1; i < len(instances); i++ {
		err = instances[i].applyBlk(ctx, lastAcceptedBlock)
		if err != nil {
			return err
		}
	}
	return nil
}

func (*Network) GenerateTx(ctx context.Context, actions []chain.Action, auth chain.AuthFactory) (*chain.Transaction, error) {
	return instances[0].GenerateTx(ctx, actions, auth)
}

// SynchronizeNetwork ensures that all the nodes on the network are at the same block height.
// this method should be called at the beginning of each test to ensure good starting point.
func (*Network) SynchronizeNetwork(ctx context.Context) error {
	// find the latest block height across the network
	var biggestHeight uint64
	var biggestHeightInstanceIndex int
	for i, instance := range instances {
		lastHeight, err := instance.vm.GetLastAcceptedHeight()
		if err != nil {
			return err
		}
		if lastHeight >= biggestHeight {
			biggestHeightInstanceIndex = i
			biggestHeight = lastHeight
		}
	}
	for i := 0; i < len(instances); i++ {
		if i == biggestHeightInstanceIndex {
			continue
		}
		vm := instances[i].vm
		for {
			height, err := vm.GetLastAcceptedHeight()
			if err != nil {
				return err
			}
			if height == biggestHeight {
				break
			}
			statefulBlock, err := instances[biggestHeightInstanceIndex].vm.GetDiskBlock(ctx, height+1)
			if err != nil {
				return err
			}
			err = vm.SetPreference(ctx, statefulBlock.ID())
			if err != nil {
				return err
			}
			blk, err := vm.ParseBlock(ctx, statefulBlock.Bytes())
			if err != nil {
				return err
			}
			err = blk.Verify(ctx)
			if err != nil {
				return err
			}
			err = blk.Accept(ctx)
			if err != nil {
				return err
			}
			if instances[i].onAccept != nil {
				instances[i].onAccept(blk)
			}
		}
	}
	return nil
}

func (i *instance) applyBlk(ctx context.Context, lastAcceptedBlock *chain.StatefulBlock) error {
	err := i.vm.SetPreference(ctx, lastAcceptedBlock.ID())
	if err != nil {
		return fmt.Errorf("applyBlk failed to set preference : %w", err)
	}
	blk, err := i.vm.ParseBlock(ctx, lastAcceptedBlock.Bytes())
	if err != nil {
		return fmt.Errorf("applyBlk failed to parse block : %w", err)
	}
	err = blk.Verify(ctx)
	if err != nil {
		return fmt.Errorf("applyBlk failed to verify block : %w", err)
	}
	err = blk.Accept(ctx)
	if err != nil {
		return fmt.Errorf("applyBlk failed to accept block : %w", err)
	}
	if i.onAccept != nil {
		i.onAccept(blk)
	}
	return nil
}

func (i *instance) confirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	errs := i.vm.Submit(ctx, true, txs)
	if len(errs) != 0 && errs[0] != nil {
		return errs[0]
	}

	expectBlk(i)(false)

	for _, tx := range txs {
		err := i.confirmTx(ctx, tx.ID())
		if err != nil {
			return err
		}
	}

	return nil
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
