// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/lineagevm/storage"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/lineagevm/consts"
)

var _ chain.Action = (*AddStudent)(nil)

type AddStudent struct {
	// Name of the professor
	ProfessorName string `json:"professorName"`

	// Name of the student
	StudentName string `json:"studentName"`
}

// ComputeUnits implements chain.Action.
func (a *AddStudent) ComputeUnits(chain.Rules) uint64 {
	// TODO: tune this
	return 1
}

// Execute implements chain.Action.
func (a *AddStudent) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	err := storage.AddStudentToState(ctx, mu, a.ProfessorName, a.StudentName)
	if err != nil {
		return nil, err
	}

	return [][]byte{}, nil
}

// GetTypeID implements chain.Action.
func (a *AddStudent) GetTypeID() uint8 {
	return mconsts.AddStudentID
}

// Marshal implements chain.Action.
func (a *AddStudent) Marshal(p *codec.Packer) {
	p.PackString(a.ProfessorName)
	p.PackString(a.StudentName)
}

func UnMarshalAddStudent(p *codec.Packer) (chain.Action, error) {
	var action AddStudent

	action.ProfessorName = p.UnpackString(false)
	action.StudentName = p.UnpackString(false)

	return &action, nil
}

// Size implements chain.Action.
func (a *AddStudent) Size() int {
	return len(a.ProfessorName) + len(a.StudentName)
}

// StateKeys implements chain.Action.
// Only need to access professor state
func (a *AddStudent) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	professorID := storage.GenerateProfessorID(a.ProfessorName)
	studentID := storage.GenerateProfessorID(a.StudentName)

	return state.Keys{
		string(storage.ProfessorStateKey(professorID)): state.All,
		string(storage.ProfessorStateKey(studentID)):   state.Read,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (a *AddStudent) StateKeysMaxChunks() []uint16 {
	// TODO: fine tune this
	return []uint16{storage.ProfessorStateChunks}
}

// ValidRange implements chain.Action.
func (a *AddStudent) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
