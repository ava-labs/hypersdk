// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/api"
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

func With() api.Option {
	return api.NewOption(Namespace, NewDefaultConfig(), func(_ api.VM, config Config) (api.Opt, error) {
		if !config.Enabled {
			return api.NewOpt(), nil
		}
		return api.WithVMAPIs(jsonRPCServerFactory{}), nil
	})
}

func WithRuntime() api.Option {
	return api.NewOption(Namespace+"runtime", *runtime.NewConfig(), func(v api.VM, cfg runtime.Config) (api.Opt, error) {
		wasmRuntime = runtime.NewRuntime(&cfg, v.Logger())
		return api.NewOpt(), nil
	})
}
