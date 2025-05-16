// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

// StoreReportID is the unique ID for the StoreReport action.
const StoreReportID = 0x01

// StoreReport is an action that stores a mapping from ReqId to Report in the blockchain state.
type StoreReport struct {
	ReqId  uint64 `serialize:"true" json:"reqId"`  // Unique identifier for the report
	Report string `serialize:"true" json:"report"` // Report content to store
}

// GetTypeID returns the unique ID for this action.
func (*StoreReport) GetTypeID() uint8 {
	return StoreReportID
}

// StateKeys defines the state keys this action will access.
func (s *StoreReport) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	// Use ReqId as the key for storing the Report
	key := strconv.FormatUint(s.ReqId, 10)
	return state.Keys{
		string(StorageKey(key)): state.Read | state.Write,
	}
}

// Execute performs the action's state transition logic.
func (s *StoreReport) Execute(
	ctx context.Context,
	r chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	actionID ids.ID,
) ([]byte, error) {
	// Validate inputs
	if len(s.Report) == 0 || len(s.Report) > 1024 { // Limit Report size to 1KB
		return nil, ErrInvalidReport
	}

	// Store the Report in the state using ReqId as the key
	key := strconv.FormatUint(s.ReqId, 10)
	if err := mu.Insert(ctx, StorageKey(key), []byte(s.Report)); err != nil {
		return nil, err
	}

	// Serialize the output
	output := &StoreReportOutput{
		Stored: true,
	}
	return output.Bytes(), nil
}

// ComputeUnits returns the computational cost of this action.
func (*StoreReport) ComputeUnits(chain.Rules) uint64 {
	return 1 // Adjust based on complexity
}

// ValidRange returns the valid timestamp range for this action.
func (*StoreReport) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1 // Valid forever
}

// Marshal serializes the action to bytes.
func (s *StoreReport) Marshal(p *codec.Packer) {
	p.PackUint64(s.ReqId)
	p.PackString(s.Report)
}

// Unmarshal deserializes the action from bytes.
func (s *StoreReport) Unmarshal(p *codec.Packer) error {
	s.ReqId = p.UnpackUint64(true)
	s.Report = p.UnpackString(true)
	if p.Err() != nil {
		return p.Err()
	}
	return nil
}

// Bytes returns the serialized bytes of the action.
func (s *StoreReport) Bytes() []byte {
	// Allocate buffer: 8 bytes for uint64 + string length (up to 1024 + 2 bytes for length prefix)
	p := codec.NewWriter(8+1026, 8+1026)
	s.Marshal(p)
	return p.Bytes()
}

// StorageKey generates the state key for a given ReqId.
func StorageKey(reqId string) []byte {
	return append([]byte("report_"), []byte(reqId)...)
}

var _ codec.Typed = (*StoreReportOutput)(nil)

// StoreReportOutput is the output of the StoreReport action.
type StoreReportOutput struct {
	Stored bool `serialize:"true" json:"stored"`
}

// GetTypeID returns the unique ID for this output.
func (*StoreReportOutput) GetTypeID() uint8 {
	return StoreReportID
}

// Marshal serializes the output to bytes.
func (o *StoreReportOutput) Marshal(p *codec.Packer) {
	p.PackBool(o.Stored)
}

// Unmarshal deserializes the output from bytes.
func (o *StoreReportOutput) Unmarshal(p *codec.Packer) error {
	o.Stored = p.UnpackBool()
	return p.Err()
}

// Bytes returns the serialized bytes of the output.
func (o *StoreReportOutput) Bytes() []byte {
	// Allocate a buffer with sufficient size (1 byte for bool)
	p := codec.NewWriter(1, 1) // Fixed size for boolean
	o.Marshal(p)
	return p.Bytes()
}

var (
	ErrInvalidReport = errors.New("report must be non-empty and less than 1KB")
)
