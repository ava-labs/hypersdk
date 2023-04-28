package rpc

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

const (
	DecisionMode byte = 0
	BlockMode    byte = 1
)

func PackBlockMessage(b *chain.StatelessBlock) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackByte(BlockMode)
	// Pack the block bytes
	p.PackBytes(b.Bytes())
	results, err := chain.MarshalResults(b.Results())
	if err != nil {
		return nil, err
	}
	// Pack the results bytes
	p.PackBytes(results)
	return p.Bytes(), p.Err()
}

func UnpackBlockMessage(
	msg []byte,
	parser chain.Parser,
) (*chain.StatefulBlock, []*chain.Result, error) {
	// Read block
	p := codec.NewReader(msg, consts.MaxInt)
	p.UnpackByte()
	var blkMsg []byte
	p.UnpackBytes(consts.MaxInt, true, &blkMsg)
	blk, err := chain.UnmarshalBlock(blkMsg, parser)
	if err != nil {
		return nil, nil, err
	}
	// Read results
	var resultsMsg []byte
	p.UnpackBytes(consts.MaxInt, true, &resultsMsg)
	results, err := chain.UnmarshalResults(resultsMsg)
	if err != nil {
		return nil, nil, err
	}
	// should be empty
	if !p.Empty() {
		return nil, nil, chain.ErrInvalidObject
	}
	return blk, results, p.Err()
}

// Could be a better place for these methods
// Packs an accepted block message
func PackAcceptedTxMessage(txID ids.ID, result *chain.Result) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackByte(DecisionMode)
	p.PackID(txID)
	p.PackBool(false)
	result.Marshal(p)
	return p.Bytes(), p.Err()
}

// Packs a removed block message
func PackRemovedTxMessage(txID ids.ID, err error) ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	p.PackByte(DecisionMode)
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
	// should be empty
	if !p.Empty() {
		return ids.Empty, nil, nil, chain.ErrInvalidObject
	}
	return txID, nil, result, p.Err()
}
