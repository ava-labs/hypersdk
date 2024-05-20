// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

type Result struct {
	// TODO: need dedicated error field because error could be returned in output?
	// TODO: assume success if no error returned by action (most go-like and allows explicit return of errors here)?
	Success bool
	Error   []byte

	// TODO: if add to end, may look like the output of a different action?
	// TODO: if are being charged, you should be told what error is
	Outputs [][][]byte

	Consumed fees.Dimensions
	Fee      uint64
}

func (r *Result) Size() int {
	outputSize := len(r.Outputs)
	for _, action := range r.Outputs {
		for _, output := range action {
			outputSize += codec.BytesLen(output) + 1 // for each output
		}
	}
	return consts.BoolLen + outputSize + fees.DimensionsLen + consts.Uint64Len
}

func (r *Result) Marshal(p *codec.Packer) error {
	p.PackBool(r.Success)
	p.PackInt(len(r.Outputs))
	for _, action := range r.Outputs {
		p.PackInt(len(action))
		for _, output := range action {
			p.PackBytes(output)
		}
	}
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
	totalOutputs := [][][]byte{}
	numActions := p.UnpackInt(false)
	for i := 0; i < numActions; i++ {
		numOutputs := p.UnpackInt(false)
		outputs := [][]byte{}
		for j := 0; j < numOutputs; j++ {
			var output []byte
			p.UnpackBytes(consts.MaxInt, false, &output)
			outputs = append(outputs, output)
		}
		totalOutputs = append(totalOutputs, outputs)
	}
	result.Outputs = totalOutputs
	if len(result.Outputs) == 0 {
		// Enforce object standardization
		result.Outputs = nil
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
