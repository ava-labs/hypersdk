// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

const Namespace = "controller"

type Config struct {
	Enabled bool `json:"enabled"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}

func With() vm.Option[struct{}] {
	return vm.NewOption[struct{}](Namespace, NewDefaultConfig(), func(v *vm.VM[struct{}], config Config) error {
		if !config.Enabled {
			return nil
		}
		vm.WithVMAPIs[struct{}](jsonRPCServerFactory{})(v)
		return nil
	})
}

func WithRuntime() vm.Option[struct{}] {
	return vm.NewOption[struct{}](Namespace+"runtime", *runtime.NewConfig(), func(v *vm.VM[struct{}], cfg runtime.Config) error {
		wasmRuntime = runtime.NewRuntime(&cfg, v.Logger())
		return nil
	})
}
