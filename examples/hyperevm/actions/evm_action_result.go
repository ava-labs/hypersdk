// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
)

var _ codec.Typed = (*EvmActionResult)(nil)

// EvmActionResult is the return type used for both EvmCall and EvmSignedCall
type EvmActionResult struct {
	Success         bool           `serialize:"true" json:"success"`
	Return          []byte         `serialize:"true" json:"return"`
	UsedGas         uint64         `serialize:"true" json:"usedGas"`
	ErrorCode       ErrorCode      `serialize:"true" json:"errorCode"`
	ContractAddress common.Address `serialize:"true" json:"contractAddress"`
	Logs            []Log          `serialize:"true" json:"logs"`

	To   common.Address `serialize:"true" json:"to"`
	From common.Address `serialize:"true" json:"from"`
}

// Since there's only one Result type, we defer to using the typeID for EvmCall
func (*EvmActionResult) GetTypeID() uint8 {
	return consts.EvmCallID
}

func (e *EvmActionResult) Bytes() []byte {
	// TODO: fine-tune these values
	p := &wrappers.Packer{
		Bytes:   make([]byte, 1024),
		MaxSize: 1024,
	}
	p.PackByte(consts.EvmCallID)
	_ = codec.LinearCodec.MarshalInto(e, p)
	return p.Bytes
}

func UnmarshalEvmActionResult(b []byte) (codec.Typed, error) {
	t := &EvmActionResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]}, // XXX: first byte is guaranteed to be the typeID by the type parser
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
