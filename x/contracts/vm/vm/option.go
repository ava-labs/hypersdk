// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/api"
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

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(_ api.VM, config Config) (vm.Opt, error) {
		if !config.Enabled {
			return vm.NewOpt(), nil
		}
		return vm.WithVMAPIs(jsonRPCServerFactory{}), nil
	})
}

func WithRuntime() vm.Option {
	return vm.NewOption(Namespace+"runtime", *runtime.NewConfig(), func(v api.VM, cfg runtime.Config) (vm.Opt, error) {
		wasmRuntime = runtime.NewRuntime(&cfg, v.Logger())
		return vm.NewOpt(), nil
	})
}
