// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"path/filepath"

	"github.com/gorilla/rpc/v2"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/examples/hyperevm/api/ethrpc"
	"github.com/ava-labs/hypersdk/examples/hyperevm/api/net"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Namespace = "ethapi"
	Endpoint  = "/rpc"
)

var _ api.HandlerFactory[api.VM] = ethAPIFactory{}

type Config struct {
	Enabled     bool   `json:"enabled"`
	BlockWindow uint64 `json:"blockWindow"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled:     true,
		BlockWindow: 1024,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), func(v api.VM, config Config) (vm.Opt, error) {
		// Creating indexer
		indexerPath := filepath.Join(v.GetDataDir(), Namespace)
		indexer, err := ethrpc.NewIndexer(indexerPath, v.GetParser(), config.BlockWindow)
		if err != nil {
			return nil, err
		}

		subscriptionFactory := &subscriptionFactory{
			indexer: indexer,
		}

		serverFactory := &ethAPIFactory{
			indexer: indexer,
		}

		return vm.NewOpt(
			vm.WithVMAPIs(serverFactory),
			vm.WithBlockSubscriptions(subscriptionFactory),
		), nil
	})
}

type ethAPIFactory struct {
	indexer *ethrpc.Indexer
}

func (e ethAPIFactory) New(vm api.VM) (api.Handler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(NewCodec(), "application/json")
	server.RegisterCodec(NewCodec(), "application/json;charset=UTF-8")

	server.RegisterService(ethrpc.NewService(vm, e.indexer))
	server.RegisterService(net.NewService(vm))

	return api.Handler{
		Handler: server,
		Path:    Endpoint,
	}, nil
}

type subscriptionFactory struct {
	indexer *ethrpc.Indexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.ExecutedBlock], error) {
	return s.indexer, nil
}
