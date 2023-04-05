// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/gorilla/websocket"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/pubsub"
)

type rpcMode int

const (
	decisions rpcMode = 0
	blocks    rpcMode = 1
)

func (vm *VM) BlocksPort() uint16 {
	return vm.blocksServer.Port()
}

// If you don't keep up, you will data
type DecisionRPCClient struct {
	conn *websocket.Conn

	wl sync.Mutex
	ll sync.Mutex
	cl sync.Once
}

func NewDecisionRPCClient(uri string) (*DecisionRPCClient, error) {
	// nil for now until we want to pass in headers
	conn, _, err := websocket.DefaultDialer.Dial(uri, nil)
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
	// the packer will be set up so that the first bytes are the tx.id
	// the next byte is a bool indicating if there is an error
	// if there is an error, the next int indicates the length of the error
	// if there is an error, than the next bytes follow the error
	// if there is no error, the next int indicates the length of the result
	// if there is no error, the next bytes follow the result
	_, msg, err := d.conn.ReadMessage()
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	// TODO: not sure if this is needed.
	if msg == nil {
		return ids.Empty, nil, nil, errors.New("nil message")
	}
	p := codec.NewReader(msg, consts.MaxInt)
	// read the txID from packer
	var txID ids.ID
	p.UnpackID(true, &txID)
	// didn't unpack id correctly
	if p.Err() != nil {
		return ids.Empty, nil, nil, p.Err()
	}
	// if packer has error
	if p.UnpackBool() {
		// rest of the bytes are the error
		len := len(p.Bytes()) - p.Offset()
		var err_bytes []byte
		p.UnpackBytes(len, true, &err_bytes)
		if p.Err() != nil {
			return ids.Empty, nil, nil, p.Err()
		}
		// convert err_bytes to error
		return ids.Empty, nil, nil, errors.New(string(err_bytes))
	}
	// unpack the result
	len := len(p.Bytes()) - p.Offset()
	var result_bytes []byte
	p.UnpackBytes(len, true, &result_bytes)
	if p.Err() != nil {
		return ids.Empty, nil, nil, p.Err()
	}
	result, err := chain.UnmarshalResult(p)
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	if !p.Empty() {
		return ids.Empty, nil, nil, chain.ErrInvalidObject
	}
	return txID, nil, result, nil
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
func (vm *VM) decisionServerCallback(msgBytes []byte, c *pubsub.Connection) []byte {
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
		return nil
	}
	// Verify tx
	sigVerify := tx.AuthAsyncVerify()
	if err := sigVerify(); err != nil {
		vm.snowCtx.Log.Error("failed to verify sig",
			zap.Error(err),
		)
		return nil
	}

	// TODO: add tx assoicated with this connection
	// vm.listeners.AddTxListener(tx, c)

	// Submit will remove from [txWaiters] if it is not added
	txID := tx.ID()
	if err := vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
		vm.snowCtx.Log.Error("failed to submit tx",
			zap.Stringer("txID", txID),
			zap.Error(err),
		)
		return nil
	}
	vm.snowCtx.Log.Debug("submitted tx", zap.Stringer("id", txID))
	return nil
}

// blockRPCServer is used by all clients to stream accepted blocks.
//
// TODO: write an RPC server that streams information about select parts of
// content instead of just entire blocks
type blockRPCServer struct {
	port     uint16
	vm       *VM
	listener net.Listener
}

type blockRPCConnection struct {
	conn net.Conn

	vm   *VM
	blks chan *chain.StatelessBlock
	id   ids.ID
}

func (r *blockRPCServer) Port() uint16 {
	return r.port
}

func (r *blockRPCServer) Run() {
	defer func() {
		_ = r.listener.Close()
	}()

	// Wait for VM to be ready before accepting connections. If we stop the VM
	// before this happens, we should return.
	if !r.vm.waitReady() {
		return
	}

	for {
		nconn, err := r.listener.Accept()
		if err != nil {
			r.vm.snowCtx.Log.Warn("unable to accept connection", zap.Error(err))
			return
		}

		// Generate connection id
		var id ids.ID
		_, err = rand.Read(id[:])
		if err != nil {
			panic(err)
		}

		// Store id
		c := make(chan *chain.StatelessBlock, r.vm.config.GetStreamingBacklogSize())
		r.vm.listeners.AddBlockListener(id, c)
		r.vm.snowCtx.Log.Info("created new block listener", zap.Stringer("id", id))

		conn := &blockRPCConnection{
			conn: nconn,

			vm:   r.vm,
			blks: c,
			id:   id,
		}

		// Nothing to read
		go conn.handleWrites()
	}
}

func (r *blockRPCServer) Close() error {
	return r.listener.Close()
}

func (c *blockRPCConnection) handleWrites() {
	defer func() {
		c.vm.metrics.blocksRPCConnections.Dec()
		_ = c.conn.Close()
		c.vm.listeners.RemoveBlockListener(c.id)
	}()

	c.vm.metrics.blocksRPCConnections.Inc()
	for {
		select {
		case blk := <-c.blks:
			if err := writeNetMessage(c.conn, blk.Bytes()); err != nil {
				c.vm.snowCtx.Log.Error("unable to send blk", zap.Error(err))
				return
			}
			results, err := chain.MarshalResults(blk.Results())
			if err != nil {
				c.vm.snowCtx.Log.Error("unable to marshal blk results", zap.Error(err))
				return
			}
			if err := writeNetMessage(c.conn, results); err != nil {
				c.vm.snowCtx.Log.Error("unable to send blk results", zap.Error(err))
				return
			}
		case <-c.vm.stop:
			return
		}
	}
}

// If you don't keep up, you will data
type BlockRPCClient struct {
	conn net.Conn

	ll sync.Mutex
	cl sync.Once
}

func NewBlockRPCClient(uri string) (*BlockRPCClient, error) {
	conn, err := net.Dial(constants.NetworkType, uri)
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

	ritem, err := readNetMessage(c.conn, false)
	if err != nil {
		return nil, nil, err
	}
	// Read block
	blk, err := chain.UnmarshalBlock(ritem, parser)
	if err != nil {
		return nil, nil, err
	}
	ritem, err = readNetMessage(c.conn, false)
	if err != nil {
		return nil, nil, err
	}
	// Read results
	results, err := chain.UnmarshalResults(ritem)
	if err != nil {
		return nil, nil, err
	}
	return blk, results, nil
}

func (c *BlockRPCClient) Close() error {
	var err error
	c.cl.Do(func() {
		err = c.conn.Close()
	})
	return err
}

func writeNetMessage(conn io.Writer, b []byte) error {
	size := len(b)
	if size == 0 {
		var buf net.Buffers = [][]byte{binary.BigEndian.AppendUint32(nil, uint32(size))}
		_, err := io.CopyN(conn, &buf, 4)
		return err
	}
	var buf net.Buffers = [][]byte{binary.BigEndian.AppendUint32(nil, uint32(size)), b}
	_, err := io.CopyN(conn, &buf, 4+int64(size))
	return err
}

func readNetMessage(conn io.Reader, limit bool) ([]byte, error) {
	lb := make([]byte, 4)
	_, err := io.ReadFull(conn, lb)
	if err != nil {
		return nil, err
	}
	l := binary.BigEndian.Uint32(lb)
	if limit && l > chain.NetworkSizeLimit {
		return nil, fmt.Errorf("%d is larger than max var size %d", l, chain.NetworkSizeLimit)
	}
	if l == 0 {
		return nil, nil
	}
	rb := make([]byte, int(l))
	_, err = io.ReadFull(conn, rb)
	if err != nil {
		return nil, err
	}
	return rb, nil
}

// TODO: pack bytes in listener file

// func (c *decisionRPCConnection) handleWrites() {
// 	defer func() {
// 		_ = c.conn.Close()
// 		// listeners will eventually be discarded when all txs are accepted or
// 		// expire
// 	}()

// 	for {
// 		select {
// 		case tx := <-c.listener:
// 			var buf net.Buffers = [][]byte{tx.TxID[:]}
// 			if _, err := io.CopyN(c.conn, &buf, consts.IDLen); err != nil {
// 				return
// 			}
// 			var errBytes []byte
// 			if err := tx.Err; err != nil {
// 				errBytes = utils.ErrBytes(err)
// 			}
// 			if err := writeNetMessage(c.conn, errBytes); err != nil {
// 				c.vm.snowCtx.Log.Error("unable to send decision err", zap.Error(err))
// 				return
// 			}
// 			var resultBytes []byte
// 			if result := tx.Result; result != nil {
// 				p := codec.NewWriter(consts.MaxInt)
// 				result.Marshal(p)
// 				if err := p.Err(); err != nil {
// 					c.vm.snowCtx.Log.Error("unable to marshal result", zap.Error(err))
// 					return
// 				}
// 				resultBytes = p.Bytes()
// 			}
// 			if err := writeNetMessage(c.conn, resultBytes); err != nil {
// 				c.vm.snowCtx.Log.Error("unable to send result", zap.Error(err))
// 				return
// 			}
// 		case <-c.vm.stop:
// 			return
// 		}
// 	}
// }
