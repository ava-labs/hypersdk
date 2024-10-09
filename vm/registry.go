// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import "github.com/ava-labs/hypersdk/chain"

type Registry struct {
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry
	outputRegistry chain.OutputRegistry
}

func NewRegistry(action chain.ActionRegistry, auth chain.AuthRegistry, output chain.OutputRegistry) *Registry {
	return &Registry{
		actionRegistry: action,
		authRegistry:   auth,
		outputRegistry: output,
	}
}

func (r *Registry) ActionRegistry() chain.ActionRegistry {
	return r.actionRegistry
}

func (r *Registry) AuthRegistry() chain.AuthRegistry {
	return r.authRegistry
}

func (r *Registry) OutputRegistry() chain.OutputRegistry {
	return r.outputRegistry
}
