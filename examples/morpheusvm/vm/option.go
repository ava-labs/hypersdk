// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/api"
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
	return api.NewOption(Namespace, NewDefaultConfig(), func(v api.VM, config Config) (api.Opt, error) {
		if !config.Enabled {
			return api.NewOpt(), nil
		}
		return api.WithVMAPIs(jsonRPCServerFactory{}), nil
	})
}
