// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	BlockMode byte = 0
	ChunkMode byte = 1
	TxMode    byte = 2

	TxSuccess uint8 = 0
	TxFailed  uint8 = 1
	TxExpired uint8 = 2
	TxInvalid uint8 = 3
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

func PackTxMessage(txID uint64, status uint8) ([]byte, error) {
	size := consts.IDLen + consts.Uint8Len
	p := codec.NewWriter(size, consts.MaxInt)
	p.PackUint64(txID)
	p.PackByte(status)
	return p.Bytes(), p.Err()
}

func UnpackTxMessage(msg []byte) (uint64, uint8, error) {
	p := codec.NewReader(msg, consts.Uint64Len+consts.BoolLen)
	txID := p.UnpackUint64(false)
	status := p.UnpackByte()
	if !p.Empty() {
		return 0, 0, chain.ErrInvalidObject
	}
	return txID, status, p.Err()
}
