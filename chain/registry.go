// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"github.com/ava-labs/hypersdk/codec"
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

func (a *AuthRegistry) LookupIndex(typeID uint8) (func(*codec.Packer) (Auth, error), bool) {
	f, found := a.authFuncs[typeID]
	return f, found
}

type ActionRegistry struct {
	actions     map[uint8]Typed
	actionFuncs map[uint8]func(*codec.Packer) (Action, error)
}

func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		actions:     make(map[uint8]Typed),
		actionFuncs: make(map[uint8]func(*codec.Packer) (Action, error)),
	}
}

func (a *ActionRegistry) Register(instance Typed, outputInstance interface{}, f func(*codec.Packer) (Action, error)) error {
	typeID := instance.GetTypeID()

	a.actions[typeID] = instance
	a.actionFuncs[typeID] = f
	return nil
}

func (a *ActionRegistry) LookupIndex(typeID uint8) (func(*codec.Packer) (Action, error), bool) {
	f, found := a.actionFuncs[typeID]
	return f, found
}

func (a *ActionRegistry) GetRegisteredTypes() []ActionPair {
	types := make([]ActionPair, 0, len(a.actions))
	for _, action := range a.actions {
		types = append(types, ActionPair{Input: action})
	}
	return types
}
