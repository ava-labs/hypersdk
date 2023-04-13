// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"net/url"
	"sync"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/gorilla/websocket"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/listeners"
	"github.com/ava-labs/hypersdk/pubsub"
)

func (vm *VM) BlocksPort() uint16 {
	return vm.config.GetBlocksPort()
}

func (vm *VM) DecisionsPort() uint16 {
	return vm.config.GetDecisionsPort()
}

// If you don't keep up, you will data
type DecisionRPCClient struct {
	conn *websocket.Conn
	wl   sync.Mutex
	ll   sync.Mutex
	cl   sync.Once
}

func NewDecisionRPCClient(uri string) (*DecisionRPCClient, error) {
	// nil for now until we want to pass in headers
	u := url.URL{Scheme: "ws", Host: uri}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	// not using resp for now
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &DecisionRPCClient{conn: conn}, nil
}

func (d *DecisionRPCClient) IssueTx(tx *chain.Transaction) error {
	d.wl.Lock()
	defer d.wl.Unlock()

	return d.conn.WriteMessage(websocket.BinaryMessage, tx.Bytes())
}

func (d *DecisionRPCClient) Listen() (ids.ID, error, *chain.Result, error) {
	d.ll.Lock()
	defer d.ll.Unlock()
	_, msg, err := d.conn.ReadMessage()
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	return listeners.UnpackTxMessage(msg)
}

func (d *DecisionRPCClient) Close() error {
	var err error
	d.cl.Do(func() {
		err = d.conn.Close()
	})
	return err
}

// decisionServerCallback is a callback function for the decision server.
// The server submits the tx to the vm and adds the tx to the vms listener for
// later retrieval.
// TODO: update return value to maybe be more useful
func (vm *VM) decisionServerCallback(msgBytes []byte, c *pubsub.Connection) {
	ctx, span := vm.tracer.Start(context.Background(), "decisionRPCServer callback")
	defer span.End()
	// Unmarshal TX
	p := codec.NewReader(msgBytes, chain.NetworkSizeLimit) // will likely be much smaller
	tx, err := chain.UnmarshalTx(p, vm.actionRegistry, vm.authRegistry)
	if err != nil {
		vm.snowCtx.Log.Error("failed to unmarshal tx",
			zap.Int("len", len(msgBytes)),
			zap.Error(err),
		)

		return
	}
	// Verify tx
	sigVerify := tx.AuthAsyncVerify()
	if err := sigVerify(); err != nil {
		vm.snowCtx.Log.Error("failed to verify sig",
			zap.Error(err),
		)
		return
	}
	// TODO: add tx associated with this connection
	vm.listeners.AddTxListener(tx, c)

	// Submit will remove from [txWaiters] if it is not added
	txID := tx.ID()
	if err := vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
		vm.snowCtx.Log.Error("failed to submit tx",
			zap.Stringer("txID", txID),
			zap.Error(err),
		)
		return
	}
	vm.snowCtx.Log.Debug("submitted tx", zap.Stringer("id", txID))
}

// If you don't keep up, you will data
type BlockRPCClient struct {
	conn *websocket.Conn

	ll sync.Mutex
	cl sync.Once
}

func NewBlockRPCClient(uri string) (*BlockRPCClient, error) {
	// nil for now until we want to pass in headers
	u := url.URL{Scheme: "ws", Host: uri}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	// not using resp for now
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &BlockRPCClient{conn: conn}, nil
}

func (c *BlockRPCClient) Listen(
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	c.ll.Lock()
	defer c.ll.Unlock()

	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return nil, nil, err
	}
	return listeners.UnpackBlockMessageBytes(msg, parser)
}

func (c *BlockRPCClient) Close() error {
	var err error
	c.cl.Do(func() {
		err = c.conn.Close()
	})
	return err
}
