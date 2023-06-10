// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	BlockMode byte = 0
	TxMode    byte = 1
)

func PackBlockMessage(b *chain.StatelessBlock) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackBytes(b.Bytes())
	results, err := chain.MarshalResults(b.Results())
	if err != nil {
		return nil, err
	}
	p.PackBytes(results)
	return p.Bytes(), p.Err()
}

func UnpackBlockMessage(
	msg []byte,
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	var blkMsg []byte
	p.UnpackBytes(-1, true, &blkMsg)
	blk, err := chain.UnmarshalBlock(blkMsg, parser)
	if err != nil {
		return nil, nil, err
	}
	var resultsMsg []byte
	p.UnpackBytes(-1, true, &resultsMsg)
	results, err := chain.UnmarshalResults(resultsMsg)
	if err != nil {
		return nil, nil, err
	}
	if !p.Empty() {
		return nil, nil, chain.ErrInvalidObject
	}
	return blk, results, p.Err()
}

// Could be a better place for these methods
// Packs an accepted block message
func PackAcceptedTxMessage(txID ids.ID, result *chain.Result) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackID(txID)
	p.PackBool(false)
	result.Marshal(p)
	return p.Bytes(), p.Err()
}

// Packs a removed block message
func PackRemovedTxMessage(txID ids.ID, err error) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackID(txID)
	p.PackBool(true)
	p.PackString(err.Error())
	return p.Bytes(), p.Err()
}

// Unpacks a tx message from [msg]. Returns the txID, an error regarding the status
// of the tx, the result of the tx, and an error if there was a
// problem unpacking the message.
func UnpackTxMessage(msg []byte) (ids.ID, error, *chain.Result, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	var txID ids.ID
	p.UnpackID(true, &txID)
	if p.UnpackBool() {
		err := p.UnpackString(true)
		return ids.Empty, errors.New(err), nil, p.Err()
	}
	result, err := chain.UnmarshalResult(p)
	if err != nil {
		return ids.Empty, nil, nil, err
	}
	if !p.Empty() {
		return ids.Empty, nil, nil, chain.ErrInvalidObject
	}
	return txID, nil, result, p.Err()
}
