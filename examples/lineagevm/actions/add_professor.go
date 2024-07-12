// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/binary"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/lineagevm/storage"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/lineagevm/consts"
)

var _ chain.Action = (*AddProfessor)(nil)

type AddProfessor struct {

	// Name of the Professor
	Name string `json:"name"`

	// Year in which the Professor received their PhD
	Year uint16 `json:"year"`

	// University from which the Professor received their PhD
	University string `json:"university"`
}

func (*AddProfessor) GetTypeID() uint8 {
	return mconsts.AddProfessorID
}

func (t *AddProfessor) StateKeys(actor codec.Address, _ ids.ID) state.Keys {

	professorID := storage.GenerateProfessorID(t.Name)

	return state.Keys{
		string(storage.ProfessorStateKey(professorID)): state.All,
	}
}

func (*AddProfessor) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.ProfessorStateChunks}
}

func (t *AddProfessor) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	professorID, err := storage.AddProfessorComplex(ctx, mu, t.Name, t.Year, t.University)
	if err != nil {
		return nil, err
	}

	professorIDString := codec.MustAddressBech32(mconsts.HRP, professorID)

	return [][]byte{[]byte(professorIDString)}, nil
}

func (*AddProfessor) ComputeUnits(chain.Rules) uint64 {
	return 1
}

func (t *AddProfessor) Size() int {
	return len(t.Name) + consts.Uint16Len + len(t.University)
}

func (t *AddProfessor) Marshal(p *codec.Packer) {
	// Pack name
	p.PackString(t.Name)
	// Pack year
	yearBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(yearBytes, t.Year)
	p.PackBytes(yearBytes)
	// Pack university
	p.PackString(t.University)
}

func UnmarshalAddProfessor(p *codec.Packer) (chain.Action, error) {
	var action AddProfessor
	
	// Unpack name
	action.Name = p.UnpackString(false)
	var yearBytes []byte = make([]byte, 2)
	// Unpack year
	p.UnpackBytes(2, false, &yearBytes)
	action.Year = binary.BigEndian.Uint16(yearBytes)
	// Unpack university
	action.University = p.UnpackString(false)

	return &action, nil
}

func (*AddProfessor) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

