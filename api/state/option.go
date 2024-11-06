// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/hypersdk/api"
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

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(_ api.VM, config Config) (vm.Opt, error) {
		if !config.Enabled {
			return vm.NewOpt(), nil
		}
		return vm.WithVMAPIs(JSONRPCStateServerFactory{}), nil
	})
}
