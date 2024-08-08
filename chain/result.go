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
	Error   []byte

	Outputs [][][]byte

	// Computing [Units] requires access to [StateManager], so it is returned
	// to make life easier for indexers.
	Units fees.Dimensions
	Fee   uint64
}

func (r *Result) Size() int {
	outputSize := consts.Uint8Len // actions
	for _, action := range r.Outputs {
		outputSize += consts.Uint8Len
		for _, output := range action {
			outputSize += codec.BytesLen(output)
		}
	}
	return consts.BoolLen + codec.BytesLen(r.Error) + outputSize + fees.DimensionsLen + consts.Uint64Len
}

func (r *Result) Marshal(p *codec.Packer) error {
	p.PackBool(r.Success)
	p.PackBytes(r.Error)
	p.PackByte(uint8(len(r.Outputs)))
	for _, outputs := range r.Outputs {
		p.PackByte(uint8(len(outputs)))
		for _, output := range outputs {
			p.PackBytes(output)
		}
	}
	p.PackFixedBytes(r.Units.Bytes())
	p.PackUint64(r.Fee)
	return nil
}

func MarshalResults(src []*Result) ([]byte, error) {
	size := consts.IntLen + codec.CummSize(src)
	p := codec.NewWriter(size, consts.MaxInt) // could be much larger than [NetworkSizeLimit]
	p.PackInt(uint32(len(src)))
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
	p.UnpackBytes(consts.MaxInt, false, &result.Error)
	outputs := [][][]byte{}
	numActions := p.UnpackByte()
	for i := uint8(0); i < numActions; i++ {
		numOutputs := p.UnpackByte()
		actionOutputs := [][]byte{}
		for j := uint8(0); j < numOutputs; j++ {
			var output []byte
			p.UnpackBytes(consts.MaxInt, false, &output)
			actionOutputs = append(actionOutputs, output)
		}
		outputs = append(outputs, actionOutputs)
	}
	result.Outputs = outputs
	consumedRaw := make([]byte, fees.DimensionsLen)
	p.UnpackFixedBytes(fees.DimensionsLen, &consumedRaw)
	units, err := fees.UnpackDimensions(consumedRaw)
	if err != nil {
		return nil, err
	}
	result.Units = units
	result.Fee = p.UnpackUint64(false)
	// Wait to check if empty until after all results are unpacked.
	return result, p.Err()
}

func UnmarshalResults(src []byte) ([]*Result, error) {
	p := codec.NewReader(src, consts.MaxInt) // could be much larger than [NetworkSizeLimit]
	items := p.UnpackInt(false)
	results := make([]*Result, items)
	for i := 0; i < int(items); i++ {
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
