// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

type Result struct {
	Success     bool
	Units       uint64
	Output      []byte
	WarpMessage *warp.UnsignedMessage
}

func (r *Result) Size() int {
	size := consts.BoolLen + consts.Uint64Len + codec.BytesLen(r.Output)
	if r.WarpMessage != nil {
		size += codec.BytesLen(r.WarpMessage.Bytes())
	} else {
		size += codec.BytesLen(nil)
	}
	return size
}

func (r *Result) Marshal(p *codec.Packer) {
	p.PackBool(r.Success)
	p.PackUint64(r.Units)
	p.PackBytes(r.Output)
	var warpBytes []byte
	if r.WarpMessage != nil {
		warpBytes = r.WarpMessage.Bytes()
	}
	p.PackBytes(warpBytes)
}

func MarshalResults(src []*Result) ([]byte, error) {
	size := consts.IntLen + codec.CummSize(src)
	p := codec.NewWriter(size, consts.MaxInt) // could be much larger than [NetworkSizeLimit]
	p.PackInt(len(src))
	for _, result := range src {
		result.Marshal(p)
	}
	return p.Bytes(), p.Err()
}

func UnmarshalResult(p *codec.Packer) (*Result, error) {
	result := &Result{
		Success: p.UnpackBool(),
		Units:   p.UnpackUint64(false),
	}
	p.UnpackBytes(consts.MaxInt, false, &result.Output)
	if len(result.Output) == 0 {
		// Enforce object standardization
		result.Output = nil
	}
	var warpMessage []byte
	p.UnpackBytes(MaxWarpMessageSize, false, &warpMessage)
	if len(warpMessage) > 0 {
		msg, err := warp.ParseUnsignedMessage(warpMessage)
		if err != nil {
			return nil, err
		}
		result.WarpMessage = msg
	}
	return result, p.Err()
}

func UnmarshalResults(src []byte) ([]*Result, error) {
	p := codec.NewReader(src, consts.MaxInt) // could be much larger than [NetworkSizeLimit]
	items := p.UnpackInt(false)
	results := make([]*Result, items)
	for i := 0; i < items; i++ {
		result, err := UnmarshalResult(p)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	if !p.Empty() {
		return nil, ErrInvalidObject
	}
	return results, nil
}
