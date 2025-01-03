// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import (
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/vm"
)

const Namespace = "tieredStorageTransactionManager"

var _ chain.TransactionManager = (*TieredStorageTransactionManager)(nil)

type Config struct {
	Epsilon                uint64 `json:"epsilon"`
	StorageReadKeyRefund   uint64 `json:"storageReadKeyRefund"`
	StorageReadValueRefund uint64 `json:"storageReadValueRefund"`
}

func NewDefaultConfig() Config {
	return Config{
		Epsilon:                100,
		StorageReadKeyRefund:   1,
		StorageReadValueRefund: 1,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(_ api.VM, config Config) (vm.Opt, error) {
	return vm.NewOpt(
		vm.WithChainOptions(
			chain.WithTransactionManagerFactory(
				NewTieredStorageTransactionManager(config),
			),
		),
		vm.WithExecutionShim(
			NewTieredStorageTransactionManager(config)(),
		),
	), nil
}

func NewTieredStorageTransactionManager(config Config) chain.TransactionManagerFactory {
	return func() chain.TransactionManager {
		return &TieredStorageTransactionManager{
			config: config,
		}
	}
}
