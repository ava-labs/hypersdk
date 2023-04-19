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
type Client struct {
	conn *websocket.Conn
	wl   sync.Mutex
	dll  sync.Mutex
	bll  sync.Mutex
	cl   sync.Once
}

// NewDecisionRPCClient creates a new client for the decision rpc server.
// Dials into the server at [uri] and returns a client.
func NewStreamingClient(uri string) (*Client, error) {
	// nil for now until we want to pass in headers
	u := url.URL{Scheme: "ws", Host: uri}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	// not using resp for now
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// IssueTx sends [tx] to the decision rpc server.
func (c *Client) IssueTx(tx *chain.Transaction) error {
	c.wl.Lock()
	defer c.wl.Unlock()

	return c.conn.WriteMessage(websocket.BinaryMessage, tx.Bytes())
}

// Listen listens for responses from the decision rpc server.
func (c *Client) ListenTx() (ids.ID, error, *chain.Result, error) {
	c.dll.Lock()
	defer c.dll.Unlock()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return ids.Empty, nil, nil, err
		}
		if msg == nil || msg[0] == listeners.DecisionMode {
			return listeners.UnpackTxMessage(msg)
		}
	}
}

// Close closes [d]'s connection to the decision rpc server.
func (c *Client) Close() error {
	var err error
	c.cl.Do(func() {
		err = c.conn.Close()
	})
	return err
}

// decisionServerCallback is a callback function for the decision server.
// The server submits the tx to the vm and adds the tx to the vms listener for
// later retrieval.
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

// Listen listens for messages from the blocks rpc server.
func (c *Client) ListenBlock(
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	c.bll.Lock()
	defer c.bll.Unlock()
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return nil, nil, err
		}
		if msg == nil || msg[0] == listeners.BlockMode {
			return listeners.UnpackBlockMessageBytes(msg, parser)
		}
	}
}
