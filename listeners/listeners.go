// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package listeners

import (
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/pubsub"
)

const (
	DecisionMode byte = 0
	BlockMode    byte = 1
)

type Transaction struct {
	TxID   ids.ID
	Result *chain.Result
	Err    error
}

type Listeners struct {
	txL         sync.Mutex
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
	s           *pubsub.Server
}

func New(s *pubsub.Server) *Listeners {
	return &Listeners{
		txListeners: map[ids.ID]*pubsub.Connections{},
		expiringTxs: emap.NewEMap[*chain.Transaction](),
		s:           s,
	}
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *Listeners) AddTxListener(tx *chain.Transaction, c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	txID := tx.ID()
	if _, ok := w.txListeners[txID]; !ok {
		w.txListeners[txID] = pubsub.NewConnections()
	}
	w.txListeners[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *Listeners) RemoveTx(txID ids.ID, err error) {
	w.txL.Lock()
	defer w.txL.Unlock()

	w.removeTx(txID, err)
}

func (w *Listeners) removeTx(txID ids.ID, err error) {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return
	}
	p := codec.NewWriter(consts.MaxInt)
	PackRemovedTxMessage(p, txID, err)
	w.s.Publish([]byte(txID.String()), listeners)
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
}

func (w *Listeners) SetMinTx(t int64) {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		w.removeTx(id, ErrExpired)
	}
}

func (w *Listeners) AcceptBlock(b *chain.StatelessBlock) {
	p := codec.NewWriter(consts.MaxInt)
	PackBlockMessageBytes(b, p)
	// Publish accepted block to all block listeners
	w.s.Publish(p.Bytes(), w.s.Conns())
	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
		p := codec.NewWriter(consts.MaxInt)
		txID := tx.ID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		PackAcceptedTxMessage(p, txID, results[i])
		w.s.Publish(
			p.Bytes(),
			listeners,
		)
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
}

// Could be a better place for these methods
// Packs an accepted block message
func PackAcceptedTxMessage(p *codec.Packer, txID ids.ID, result *chain.Result) {
	p.PackByte(DecisionMode)
	p.PackID(txID)
	p.PackBool(false)
	result.Marshal(p)
}

// Packs a removed block message
func PackRemovedTxMessage(p *codec.Packer, txID ids.ID, err error) {
	p.PackByte(DecisionMode)
	p.PackID(txID)
	p.PackBool(true)
	p.PackString(err.Error())
}

// Unpacks a tx message from [msg]. Returns the txID, an error regarding the status
// of the tx, the result of the tx, and an error if there was a
// problem unpacking the message.
func UnpackTxMessage(msg []byte) (ids.ID, error, *chain.Result, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	p.UnpackByte()
	// read the txID from packer
	var txID ids.ID
	p.UnpackID(true, &txID)
	if p.UnpackBool() {
		err := p.UnpackString(true)
		if p.Err() != nil {
			return ids.Empty, nil, nil, p.Err()
		}
		// convert err_bytes to error
		return ids.Empty, errors.New(err), nil, nil
	}
	// unpack the result
	result, err := chain.UnmarshalResult(p)
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	// packer had an error
	if p.Err() != nil {
		return ids.Empty, nil, nil, p.Err()
	}
	// should be empty
	if !p.Empty() {
		return ids.Empty, nil, nil, chain.ErrInvalidObject
	}
	return txID, nil, result, nil
}

func PackBlockMessageBytes(b *chain.StatelessBlock, p *codec.Packer) {
	p.PackByte(BlockMode)
	// Pack the block bytes
	p.PackBytes(b.Bytes())
	results, err := chain.MarshalResults(b.Results())
	if err != nil {
		return
	}
	// Pack the results bytes
	p.PackBytes(results)
}

func UnpackBlockMessageBytes(
	msg []byte,
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	// Read block
	p := codec.NewReader(msg, consts.MaxInt)
	p.UnpackByte()
	var blkMsg []byte
	p.UnpackBytes(-1, false, &blkMsg)
	blk, err := chain.UnmarshalBlock(blkMsg, parser)
	if err != nil {
		return nil, nil, err
	}
	// Read results
	var resultsMsg []byte
	p.UnpackBytes(-1, true, &resultsMsg)
	results, err := chain.UnmarshalResults(resultsMsg)
	if err != nil {
		return nil, nil, err
	}
	// packer had an error
	if p.Err() != nil {
		return nil, nil, p.Err()
	}
	return blk, results, nil
}
