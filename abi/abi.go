// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"fmt"

	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type ABI struct {
	ActionsSpec []*canoto.Spec `serialize:"true" json:"actionsSpec"`
	OutputsSpec []*canoto.Spec `serialize:"true" json:"outputsSpec"`

	ActionTypes []codec.TypedStruct `serialize:"true" json:"actionTypes"`
	OutputTypes []codec.TypedStruct `serialize:"true" json:"outputTypes"`
}

func NewABI(actionParser *codec.CanotoParser[chain.Action], outputParser *codec.CanotoParser[codec.Typed]) ABI {
	return ABI{
		ActionsSpec: actionParser.GetRegisteredTypes(),
		OutputsSpec: outputParser.GetRegisteredTypes(),
		ActionTypes: actionParser.GetTypedStructs(),
		OutputTypes: outputParser.GetTypedStructs(),
	}
}

func (t ABI) CalculateCanotoSpec() {
	for i := range t.ActionsSpec {
		t.ActionsSpec[i].CalculateCanotoCache()
	}
	for i := range t.OutputsSpec {
		t.OutputsSpec[i].CalculateCanotoCache()
	}
}

func (t ABI) FindActionSpecByName(name string) (*canoto.Spec, bool) {
	for _, spec := range t.ActionsSpec {
		if spec.Name == name {
			return spec, true
		}
	}
	return nil, false
}

func (t ABI) FindOutputSpecByID(id uint8) (*canoto.Spec, bool) {
	var (
		typ   codec.TypedStruct
		found bool
	)
	for _, types := range t.OutputTypes {
		if types.ID == id {
			typ = types
			found = true
			break
		}
	}

	if !found {
		return nil, false
	}

	for _, spec := range t.OutputsSpec {
		if spec.Name == typ.Name {
			return spec, true
		}
	}

	return nil, false
}

func (t ABI) GetActionID(name string) (uint8, error) {
	for _, spec := range t.ActionTypes {
		if spec.Name == name {
			return spec.ID, nil
		}
	}
	return 0, fmt.Errorf("action %s not found", name)
}

func (t ABI) GetOutputID(name string) (uint8, error) {
	for _, spec := range t.OutputTypes {
		if spec.Name == name {
			return spec.ID, nil
		}
	}
	return 0, fmt.Errorf("output %s not found", name)
}
