// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
)

type Result struct {
	Success bool `canoto:"bool,1"`
	// XXX: do not couple state transition logic to exact error values
	Error []byte `canoto:"bytes,2"`

	Outputs [][]byte `canoto:"repeated bytes,3"`

	// Computing [Units] requires access to [StateManager], so it is returned
	// to make life easier for indexers.
	Units fees.Dimensions `canoto:"fixed repeated fint64,4"`
	Fee   uint64          `canoto:"fint64,5"`

	canotoData canotoData_Result
}

type ResultJSON struct {
	Success bool        `json:"success"`
	Error   codec.Bytes `json:"error"`

	Outputs []codec.Bytes   `json:"outputs"`
	Units   fees.Dimensions `json:"units"`
	Fee     uint64          `json:"fee"`
}

func (r *Result) MarshalJSON() ([]byte, error) {
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

func (r *Result) Marshal() []byte {
	return r.MarshalCanoto()
}

func UnmarshalResult(src []byte) (*Result, error) {
	result := new(Result)
	if err := result.UnmarshalCanoto(src); err != nil {
		return nil, err
	}
	result.CalculateCanotoCache() // TODO: remove this call after canoto guarantees canotoData is populated during unmarshal
	return result, nil
}

type ExecutionResults struct {
	Results       []*Result       `canoto:"repeated field,1"        json:"results"`
	UnitPrices    fees.Dimensions `canoto:"fixed repeated fint64,2" json:"unitPrices"`
	UnitsConsumed fees.Dimensions `canoto:"fixed repeated fint64,3" json:"unitsConsumed"`

	canotoData canotoData_ExecutionResults
}

func NewExecutionResults(
	results []*Result,
	unitPrices fees.Dimensions,
	unitsConsumed fees.Dimensions,
) *ExecutionResults {
	return &ExecutionResults{
		Results:       results,
		UnitPrices:    unitPrices,
		UnitsConsumed: unitsConsumed,
	}
}

func (e *ExecutionResults) Marshal() []byte {
	return e.MarshalCanoto()
}

func ParseExecutionResults(bytes []byte) (*ExecutionResults, error) {
	results := &ExecutionResults{}
	if err := results.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	return results, nil
}
