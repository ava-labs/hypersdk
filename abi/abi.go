// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"github.com/StephenButtolph/canoto"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type ABI struct {
	ActionsSpec []*canoto.Spec `canoto:"repeated pointer,1" json:"actionsSpec"`
	OutputsSpec []*canoto.Spec `canoto:"repeated pointer,2" json:"outputsSpec"`

	ActionTypes []codec.TypedStruct `canoto:"repeated value,3" json:"actionTypes"`
	OutputTypes []codec.TypedStruct `canoto:"repeated value,4" json:"outputTypes"`

	canotoData canotoData_ABI
}

func NewABI(actionParser *codec.CanotoParser[chain.Action], outputParser *codec.CanotoParser[codec.Typed]) *ABI {
	return &ABI{
		ActionsSpec: actionParser.GetRegisteredTypes(),
		OutputsSpec: outputParser.GetRegisteredTypes(),
		ActionTypes: actionParser.GetTypedStructs(),
		OutputTypes: outputParser.GetTypedStructs(),
	}
}

func (t *ABI) CalculateCanotoSpec() {
	for i := range t.ActionsSpec {
		t.ActionsSpec[i].CalculateCanotoCache()
	}
	for i := range t.OutputsSpec {
		t.OutputsSpec[i].CalculateCanotoCache()
	}
}

func (t *ABI) FindActionSpecByName(name string) (*canoto.Spec, bool) {
	for _, spec := range t.ActionsSpec {
		if spec.Name == name {
			return spec, true
		}
	}
	return nil, false
}

func (t *ABI) FindOutputSpecByID(id uint8) (*canoto.Spec, bool) {
	var (
		typeName string
		found    bool
	)
	for i := range t.OutputTypes {
		if t.OutputTypes[i].ID == id {
			typeName = t.OutputTypes[i].Name
			found = true
			break
		}
	}

	if !found {
		return nil, false
	}

	for _, spec := range t.OutputsSpec {
		if spec.Name == typeName {
			return spec, true
		}
	}

	return nil, false
}

func (t *ABI) GetActionID(name string) (uint8, bool) {
	for i := range t.ActionTypes {
		if t.ActionTypes[i].Name == name {
			return t.ActionTypes[i].ID, true
		}
	}
	return 0, false
}

func (t *ABI) GetOutputID(name string) (uint8, bool) {
	for i := range t.OutputTypes {
		if t.OutputTypes[i].Name == name {
			return t.OutputTypes[i].ID, true
		}
	}
	return 0, false
}
