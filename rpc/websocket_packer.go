// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	BlockMode byte = 0
	TxMode    byte = 1
)

func PackBlockMessage(b *chain.StatelessRootBlock) ([]byte, error) {
	size := consts.IntLen + len(b.Bytes()) + consts.IntLen
	txBlocks := b.GetTxBlocks()
	res := make([][]byte, len(txBlocks))
	for i, txBlock := range txBlocks {
		size += consts.IntLen + len(txBlock.Bytes())
		results, err := chain.MarshalResults(txBlock.Results())
		if err != nil {
			return nil, err
		}
		size += len(results)
		res[i] = results
	}
	p := codec.NewWriter(size, consts.MaxInt)
	p.PackBytes(b.Bytes())
	p.PackInt(len(txBlocks))
	for i, txBlock := range txBlocks {
		p.PackBytes(txBlock.Bytes())
		p.PackBytes(res[i])
	}
	return p.Bytes(), p.Err()
}

func UnpackBlockMessage(
	msg []byte,
	parser chain.Parser,
) (*chain.RootBlock, []*chain.TxBlock, []*chain.Result, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	var blkMsg []byte
	p.UnpackBytes(-1, true, &blkMsg)
	blk, err := chain.UnmarshalRootBlock(blkMsg, parser)
	if err != nil {
		return nil, nil, nil, err
	}
	txBlkCount := p.UnpackInt(false) // genesis blk
	txBlks := []*chain.TxBlock{}
	results := []*chain.Result{}
	for i := 0; i < txBlkCount; i++ {
		var txBlkMsg []byte
		p.UnpackBytes(-1, true, &txBlkMsg)
		txBlk, err := chain.UnmarshalTxBlock(txBlkMsg, parser)
		if err != nil {
			return nil, nil, nil, err
		}
		txBlks = append(txBlks, txBlk)
		var resultsMsg []byte
		p.UnpackBytes(-1, true, &resultsMsg)
		result, err := chain.UnmarshalResults(resultsMsg)
		if err != nil {
			return nil, nil, nil, err
		}
		results = append(results, result...)
	}
	if !p.Empty() {
		return nil, nil, nil, chain.ErrInvalidObject
	}
	return blk, txBlks, results, p.Err()
}

// Could be a better place for these methods
// Packs an accepted block message
func PackAcceptedTxMessage(txID ids.ID, result *chain.Result) ([]byte, error) {
	p := codec.NewWriter(consts.IDLen+1+result.Size(), consts.MaxInt)
	p.PackID(txID)
	p.PackBool(false)
	result.Marshal(p)
	return p.Bytes(), p.Err()
}

// Packs a removed block message
func PackRemovedTxMessage(txID ids.ID, err error) ([]byte, error) {
	errString := err.Error()
	p := codec.NewWriter(consts.IDLen+1+consts.IDLen+len(errString), consts.MaxInt)
	p.PackID(txID)
	p.PackBool(true)
	p.PackString(errString)
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
