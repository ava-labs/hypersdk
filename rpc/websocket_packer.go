// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

const (
	BlockMode byte = 0
	TxMode    byte = 1
)

func PackBlockMessage(b *chain.StatelessBlock) ([]byte, error) {
	results := b.Results()
	size := codec.BytesLen(b.Bytes()) + consts.IntLen + codec.CummSize(results) + fees.DimensionsLen
	p := codec.NewWriter(size, consts.MaxInt)
	p.PackBytes(b.Bytes())
	mresults, err := chain.MarshalResults(results)
	if err != nil {
		return nil, err
	}
	p.PackBytes(mresults)
	p.PackFixedBytes(b.FeeManager().UnitPrices().Bytes())
	return p.Bytes(), p.Err()
}

func UnpackBlockMessage(
	msg []byte,
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, fees.Dimensions, error) {
	p := codec.NewReader(msg, consts.MaxInt)
	var blkMsg []byte
	p.UnpackBytes(-1, true, &blkMsg)
	blk, err := chain.UnmarshalBlock(blkMsg, parser)
	if err != nil {
		return nil, nil, fees.Dimensions{}, err
	}
	var resultsMsg []byte
	p.UnpackBytes(-1, true, &resultsMsg)
	results, err := chain.UnmarshalResults(resultsMsg)
	if err != nil {
		return nil, nil, fees.Dimensions{}, err
	}
	pricesMsg := make([]byte, fees.DimensionsLen)
	p.UnpackFixedBytes(fees.DimensionsLen, &pricesMsg)
	prices, err := fees.UnpackDimensions(pricesMsg)
	if err != nil {
		return nil, nil, fees.Dimensions{}, err
	}
	if !p.Empty() {
		return nil, nil, fees.Dimensions{}, chain.ErrInvalidObject
	}
	return blk, results, prices, p.Err()
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
