// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
)

type Result struct {
	Success bool
	Error   []byte

	Outputs [][]byte

	// Computing [Units] requires access to [StateManager], so it is returned
	// to make life easier for indexers.
	Units fees.Dimensions
	Fee   uint64
}

type ResultJSON struct {
	Success bool        `json:"success"`
	Error   codec.Bytes `json:"error"`

	Outputs []codec.Bytes   `json:"outputs"`
	Units   fees.Dimensions `json:"units"`
	Fee     uint64          `json:"fee"`
}

func (r Result) MarshalJSON() ([]byte, error) {
	outputs := make([]codec.Bytes, len(r.Outputs))
	for i, output := range r.Outputs {
		outputs[i] = output
	}
	resultJSON := ResultJSON{
		Success: r.Success,
		Error:   r.Error,
		Outputs: outputs,
		Units:   r.Units,
		Fee:     r.Fee,
	}

	return json.Marshal(resultJSON)
}

func (r *Result) UnmarshalJSON(data []byte) error {
	var resultJSON ResultJSON
	if err := json.Unmarshal(data, &resultJSON); err != nil {
		return err
	}

	r.Success = resultJSON.Success
	r.Error = resultJSON.Error
	r.Outputs = make([][]byte, len(resultJSON.Outputs))
	for i, output := range resultJSON.Outputs {
		r.Outputs[i] = output
	}
	r.Units = resultJSON.Units
	r.Fee = resultJSON.Fee
	return nil
}

func (r *Result) Size() int {
	outputSize := consts.Uint8Len // actions
	for _, actionOutput := range r.Outputs {
		outputSize += codec.BytesLen(actionOutput)
	}
	return consts.BoolLen + codec.BytesLen(r.Error) + outputSize + fees.DimensionsLen + consts.Uint64Len
}

func (r *Result) Marshal(p *codec.Packer) error {
	p.PackBool(r.Success)
	p.PackBytes(r.Error)
	p.PackByte(uint8(len(r.Outputs)))
	for _, actionOutput := range r.Outputs {
		p.PackBytes(actionOutput)
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
	outputs := [][]byte{}
	numActions := p.UnpackByte()
	for i := uint8(0); i < numActions; i++ {
		var output []byte
		p.UnpackBytes(consts.MaxInt, false, &output)
		outputs = append(outputs, output)
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
	for i := uint32(0); i < items; i++ {
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
