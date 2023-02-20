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

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/ips"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/listeners"
	"github.com/ava-labs/hypersdk/utils"
)

type rpcMode int

const (
	decisions rpcMode = 0
	blocks    rpcMode = 1
)

func (vm *VM) DecisionsPort() uint16 {
	return vm.decisionsServer.Port()
}

func (vm *VM) BlocksPort() uint16 {
	return vm.blocksServer.Port()
}

type rpcServer interface {
	Port() uint16 // randomly chosen if :0 provided
	Run()
	Close() error
}

func newRPC(vm *VM, mode rpcMode, port uint16) (rpcServer, error) {
	listener, err := net.Listen(constants.NetworkType, fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	addr := listener.Addr().String()
	ipPort, err := ips.ToIPPort(addr)
	if err != nil {
		return nil, err
	}

	switch mode {
	case decisions:
		return &decisionRPCServer{
			port:     ipPort.Port,
			vm:       vm,
			listener: listener,
		}, nil
	case blocks:
		return &blockRPCServer{
			port:     ipPort.Port,
			vm:       vm,
			listener: listener,
		}, nil
	}
	return nil, fmt.Errorf("%d is not a valid rpc mode", mode)
}

type decisionRPCServer struct {
	port     uint16
	vm       *VM
	listener net.Listener
}

type decisionRPCConnection struct {
	conn net.Conn

	vm       *VM
	listener listeners.TxListener
}

func (r *decisionRPCServer) Port() uint16 {
	return r.port
}

func (r *decisionRPCServer) Run() {
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
			return
		}

		conn := &decisionRPCConnection{
			conn:     nconn,
			vm:       r.vm,
			listener: make(chan *listeners.Transaction, r.vm.config.GetStreamingBacklogSize()),
		}

		go conn.handleReads()
		go conn.handleWrites()
	}
}

func (r *decisionRPCServer) Close() error {
	return r.listener.Close()
}

func (c *decisionRPCConnection) handleReads() {
	defer func() {
		c.vm.metrics.decisionsRPCConnections.Dec()
		_ = c.conn.Close()
	}()

	c.vm.metrics.decisionsRPCConnections.Inc()
	for {
		msgBytes, err := readNetMessage(c.conn, true)
		if err != nil {
			return
		}

		ctx, span := c.vm.tracer.Start(context.Background(), "decisionRPCServer.handleReads")
		p := codec.NewReader(msgBytes, chain.NetworkSizeLimit) // will likely be much smaller
		tx, err := chain.UnmarshalTx(p, c.vm.actionRegistry, c.vm.authRegistry)
		if err != nil {
			c.vm.snowCtx.Log.Error("failed to unmarshal tx",
				zap.Int("len", len(msgBytes)),
				zap.Error(err),
			)
			return
		}

		sigVerify, err := tx.Init(ctx, c.vm.actionRegistry, c.vm.authRegistry)
		if err != nil {
			c.vm.snowCtx.Log.Error("failed to initialize tx",
				zap.Error(err),
			)
			return
		}
		if err := sigVerify(); err != nil {
			c.vm.snowCtx.Log.Error("failed to verify sig",
				zap.Error(err),
			)
			return
		}
		c.vm.listeners.AddTxListener(tx, c.listener)

		// Submit will remove from [txWaiters] if it is not added
		txID := tx.ID()
		if err := c.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
			c.vm.snowCtx.Log.Error("failed to submit tx",
				zap.Stringer("txID", txID),
				zap.Error(err),
			)
			continue
		}
		c.vm.snowCtx.Log.Debug("submitted tx", zap.Stringer("id", txID))
		span.End()
	}
}

func (c *decisionRPCConnection) handleWrites() {
	defer func() {
		_ = c.conn.Close()
		// listeners will eventually be discarded when all txs are accepted or
		// expire
	}()

	for {
		select {
		case tx := <-c.listener:
			var buf net.Buffers = [][]byte{tx.TxID[:]}
			if _, err := io.CopyN(c.conn, &buf, consts.IDLen); err != nil {
				return
			}
			var errBytes []byte
			if err := tx.Err; err != nil {
				errBytes = utils.ErrBytes(err)
			}
			if err := writeNetMessage(c.conn, errBytes); err != nil {
				c.vm.snowCtx.Log.Error("unable to send decision err", zap.Error(err))
				return
			}
			var resultBytes []byte
			if result := tx.Result; result != nil {
				p := codec.NewWriter(consts.MaxInt)
				result.Marshal(p)
				if err := p.Err(); err != nil {
					c.vm.snowCtx.Log.Error("unable to marshal result", zap.Error(err))
					return
				}
				resultBytes = p.Bytes()
			}
			if err := writeNetMessage(c.conn, resultBytes); err != nil {
				c.vm.snowCtx.Log.Error("unable to send result", zap.Error(err))
				return
			}
		case <-c.vm.stop:
			return
		}
	}
}

// If you don't keep up, you will data
type DecisionRPCClient struct {
	conn net.Conn
}

func NewDecisionRPCClient(uri string) (*DecisionRPCClient, error) {
	conn, err := net.Dial(constants.NetworkType, uri)
	if err != nil {
		return nil, err
	}
	return &DecisionRPCClient{conn}, nil
}

func (d *DecisionRPCClient) IssueTx(tx *chain.Transaction) error {
	return writeNetMessage(d.conn, tx.Bytes())
}

func (d *DecisionRPCClient) Listen() (ids.ID, error, *chain.Result, error) {
	var txID ids.ID
	if _, err := io.ReadFull(d.conn, txID[:]); err != nil {
		return ids.Empty, nil, nil, err
	}
	errBytes, err := readNetMessage(d.conn, false)
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	var decisionsErr error
	if len(errBytes) > 0 {
		decisionsErr = errors.New(string(errBytes))
	}
	resultBytes, err := readNetMessage(d.conn, false)
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	var result *chain.Result
	if len(resultBytes) > 0 {
		p := codec.NewReader(resultBytes, consts.MaxInt)
		result, err = chain.UnmarshalResult(p)
		if err != nil {
			return ids.Empty, nil, nil, err
		}
		if !p.Empty() {
			return ids.Empty, nil, nil, chain.ErrInvalidObject
		}
	}
	return txID, decisionsErr, result, nil
}

func (d *DecisionRPCClient) Close() error {
	return d.conn.Close()
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
}

func NewBlockRPCClient(uri string) (*BlockRPCClient, error) {
	conn, err := net.Dial(constants.NetworkType, uri)
	if err != nil {
		return nil, err
	}
	return &BlockRPCClient{conn}, nil
}

func (c *BlockRPCClient) Listen(
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	ritem, err := readNetMessage(c.conn, false)
	if err != nil {
		return nil, nil, err
	}
	// Read block
	blk, err := chain.UnmarshalBlock(ritem, parser)
	if err != nil {
		return nil, nil, err
	}
	// Initialize hashes
	actionRegistry, authRegistry := parser.Registry()
	for _, tx := range blk.Txs {
		_, err := tx.Init(
			context.TODO(),
			actionRegistry,
			authRegistry,
		) // we don't re-verify signatures
		if err != nil {
			return nil, nil, err
		}
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
	return c.conn.Close()
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
