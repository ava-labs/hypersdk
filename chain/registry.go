// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/hypersdk/codec"

	reflect "reflect"
)

type Typed interface {
	GetTypeID() uint8
}

type AuthRegistry struct {
	auths     map[uint8]Typed
	authFuncs map[uint8]func(*codec.Packer) (Auth, error)
}

func NewAuthRegistry() *AuthRegistry {
	return &AuthRegistry{
		auths:     make(map[uint8]Typed),
		authFuncs: make(map[uint8]func(*codec.Packer) (Auth, error)),
	}
}

func (a *AuthRegistry) Register(instance Typed, f func(*codec.Packer) (Auth, error)) error {
	typeID := instance.GetTypeID()

	a.auths[typeID] = instance
	a.authFuncs[typeID] = f
	return nil
}

func (a *AuthRegistry) LookupUnmarshalFunc(typeID uint8) (func(*codec.Packer) (Auth, error), bool) {
	f, found := a.authFuncs[typeID]
	return f, found
}

type ActionRegistry struct {
	actions         map[uint8]Typed
	actionFuncs     map[uint8]func(*codec.Packer) (Action, error)
	outputInstances map[uint8]interface{}
}

func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions:         make(map[uint8]Typed),
		actionFuncs:     make(map[uint8]func(*codec.Packer) (Action, error)),
		outputInstances: make(map[uint8]interface{}),
	}
}

func (a *ActionRegistry) Register(instance Typed, outputInstance interface{}, f func(*codec.Packer) (Action, error)) error {
	typeID := instance.GetTypeID()

	if f == nil {
		instanceType := reflect.TypeOf(instance).Elem()
		f = func(p *codec.Packer) (Action, error) {
			t := reflect.New(instanceType).Interface().(Action)
			err := codec.LinearCodec.UnmarshalFrom(p.Packer, t)
			return t, err
		}
	}

	a.actions[typeID] = instance
	a.actionFuncs[typeID] = f
	a.outputInstances[typeID] = outputInstance
	return nil
}

func (a *ActionRegistry) LookupUnmarshalFunc(typeID uint8) (func(*codec.Packer) (Action, error), bool) {
	f, found := a.actionFuncs[typeID]
	return f, found
}

// func (a *ActionRegistry) UnmarshalJSON(typeID uint8, data []byte) (Action, error) {
// 	actionInstance, ok := a.actions[typeID]
// 	if !ok {
// 		return nil, fmt.Errorf("action type %d not found", typeID)
// 	}

// 	actionType := reflect.TypeOf(actionInstance).Elem()
// 	action := reflect.New(actionType).Interface().(Action)
// 	if err := json.Unmarshal(data, action); err != nil {
// 		return nil, fmt.Errorf("failed to unmarshal action: %w", err)
// 	}
// 	return action, nil
// }

func (a *ActionRegistry) UnmarshalOutputs(typeID uint8, outputs [][]byte) ([]interface{}, error) {
	outputInstance, ok := a.outputInstances[typeID]
	if !ok {
		return nil, fmt.Errorf("output type %d not found", typeID)
	}

	outputType := reflect.TypeOf(outputInstance)
	if outputType.Kind() == reflect.Ptr {
		outputType = outputType.Elem()
	}

	result := make([]interface{}, len(outputs))
	for i, outputBytes := range outputs {
		output := reflect.New(outputType).Interface()
		err := codec.LinearCodec.Unmarshal(outputBytes, output)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal output: %w", err)
		}
		result[i] = output
	}
	return result, nil
}

func (a *ActionRegistry) GetRegisteredTypes() []ActionPair {
	types := make([]ActionPair, 0, len(a.actions))
	for typeID, action := range a.actions {
		types = append(types, ActionPair{Input: action, Output: a.outputInstances[typeID]})
	}
	return types
}
