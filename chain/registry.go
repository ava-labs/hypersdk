// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "github.com/ava-labs/hypersdk/codec"

type Registry interface {
	ActionRegistry() *codec.TypeParser[Action]
	AuthRegistry() *codec.TypeParser[Auth]
	OutputRegistry() *codec.TypeParser[codec.Typed]
}

type registry struct {
	actionRegistry *codec.TypeParser[Action]
	authRegistry   *codec.TypeParser[Auth]
	outputRegistry *codec.TypeParser[codec.Typed]
}

func NewRegistry(action *codec.TypeParser[Action], auth *codec.TypeParser[Auth], output *codec.TypeParser[codec.Typed]) Registry {
	return &registry{
		actionRegistry: action,
		authRegistry:   auth,
		outputRegistry: output,
	}
}

func (r *registry) ActionRegistry() *codec.TypeParser[Action] {
	return r.actionRegistry
}

func (r *registry) AuthRegistry() *codec.TypeParser[Auth] {
	return r.authRegistry
}

func (r *registry) OutputRegistry() *codec.TypeParser[codec.Typed] {
	return r.outputRegistry
}
