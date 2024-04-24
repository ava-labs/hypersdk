// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

type Result struct {
	Success bool
	Output  []byte

	Consumed fees.Dimensions
	Fee      uint64
}

func (r *Result) Size() int {
	return consts.BoolLen + codec.BytesLen(r.Output) + fees.DimensionsLen + consts.Uint64Len
}

func (r *Result) Marshal(p *codec.Packer) error {
	p.PackBool(r.Success)
	p.PackBytes(r.Output)
	p.PackFixedBytes(r.Consumed.Bytes())
	p.PackUint64(r.Fee)
	return nil
}

func MarshalResults(src []*Result) ([]byte, error) {
	size := consts.IntLen + codec.CummSize(src)
	p := codec.NewWriter(size, consts.MaxInt) // could be much larger than [NetworkSizeLimit]
	p.PackInt(len(src))
	for _, result := range src {
		if err := result.Marshal(p); err != nil {
			return nil, err
		}
	}
	return p.Bytes(), p.Err()
}

func UnmarshalResult(p *codec.Packer) (*Result, error) {
	result := &Result{
		Success: p.UnpackBool(),
	}
	p.UnpackBytes(consts.MaxInt, false, &result.Output)
	if len(result.Output) == 0 {
		// Enforce object standardization
		result.Output = nil
	}
	consumedRaw := make([]byte, fees.DimensionsLen)
	p.UnpackFixedBytes(fees.DimensionsLen, &consumedRaw)
	consumed, err := fees.UnpackDimensions(consumedRaw)
	if err != nil {
		return nil, err
	}
	result.Consumed = consumed
	result.Fee = p.UnpackUint64(false)
	if !p.Empty() {
		return nil, p.Err()
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
