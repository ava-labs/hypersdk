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
	ChunkMode byte = 1
	TxMode    byte = 2
)

func PackChunkMessage(block uint64, c *chain.FilteredChunk, results []*chain.Result) ([]byte, error) {
	chunkBytes, err := c.Marshal()
	if err != nil {
		return nil, err
	}
	size := consts.Uint64Len + codec.BytesLen(chunkBytes) + consts.IntLen + codec.CummSize(results)
	p := codec.NewWriter(size, consts.MaxInt)
	p.PackUint64(block)
	p.PackBytes(chunkBytes)
	resultsBytes, err := chain.MarshalResults(results)
	if err != nil {
		return nil, err
	}
	p.PackBytes(resultsBytes)
	return p.Bytes(), p.Err()
}

func UnpackChunkMessage(
	msg []byte,
	parser chain.Parser,
) (uint64, *chain.FilteredChunk, []*chain.Result, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	height := p.UnpackUint64(false)
	var chunkBytes []byte
	p.UnpackBytes(-1, true, &chunkBytes)
	chunk, err := chain.UnmarshalFilteredChunk(chunkBytes, parser)
	if err != nil {
		return 0, nil, nil, err
	}
	var resultsBytes []byte
	p.UnpackBytes(-1, true, &resultsBytes)
	results, err := chain.UnmarshalResults(resultsBytes)
	if err != nil {
		return 0, nil, nil, err
	}
	if !p.Empty() {
		return 0, nil, nil, chain.ErrInvalidObject
	}
	return height, chunk, results, p.Err()
}

// Could be a better place for these methods
// Packs an accepted block message
func PackAcceptedTxMessage(txID ids.ID, result *chain.Result) ([]byte, error) {
	size := consts.IDLen + consts.BoolLen + result.Size()
	p := codec.NewWriter(size, consts.MaxInt)
	p.PackID(txID)
	p.PackBool(false)
	if err := result.Marshal(p); err != nil {
		return nil, err
	}
	return p.Bytes(), p.Err()
}

// Packs a removed block message
func PackRemovedTxMessage(txID ids.ID, err error) ([]byte, error) {
	errString := err.Error()
	size := consts.IDLen + consts.BoolLen + codec.StringLen(errString)
	p := codec.NewWriter(size, consts.MaxInt)
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
