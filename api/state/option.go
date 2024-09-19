// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/vm"
)

const Namespace = "corestate"

type Config struct {
	Enabled bool `json:"enabled"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}

func With[T chain.RuntimeInterface]() vm.Option[T] {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(v *vm.VM[T], config Config) error {
		if !config.Enabled {
			return nil
		}
		vm.WithVMAPIs[T](JSONRPCStateServerFactory[T]{})(v)
		return nil
	})
}
