// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

type TestNetwork interface {
	ConfirmTxs(context.Context, []*chain.Transaction) error
	GenerateTx(context.Context, []chain.Action, chain.AuthFactory) (*chain.Transaction, error)
	URIs() []string
	Configuration() TestNetworkConfiguration
}

// TestNetworkConfiguration is an interface, implemented by VM-specific tests
// to store information regarding the test network prior to it's invocation, and
// retrieve it during execution. All implementations must be thread-safe.
type TestNetworkConfiguration interface {
	GenesisBytes() []byte
	Name() string
	Parser() chain.Parser
	AuthFactories() []chain.AuthFactory
}

// DefaultTestNetworkConfiguration struct is the common test configuration that a test framework would need to provide
// in order to deploy a network. A test would typically embed this as part of it's network configuration structure.
type DefaultTestNetworkConfiguration struct {
	genesisBytes  []byte
	name          string
	parser        chain.Parser
	authFactories []chain.AuthFactory
}

func (d DefaultTestNetworkConfiguration) GenesisBytes() []byte {
	return d.genesisBytes
}

func (d DefaultTestNetworkConfiguration) Name() string {
	return d.name
}

func (d DefaultTestNetworkConfiguration) Parser() chain.Parser {
	return d.parser
}

func (d DefaultTestNetworkConfiguration) AuthFactories() []chain.AuthFactory {
	return d.authFactories
}

// NewDefaultTestNetworkConfiguration creates a new DefaultTestNetworkConfiguration object.
func NewDefaultTestNetworkConfiguration(genesisBytes []byte, name string, parser chain.Parser, authFactories []chain.AuthFactory) DefaultTestNetworkConfiguration {
	return DefaultTestNetworkConfiguration{
		genesisBytes:  genesisBytes,
		name:          name,
		parser:        parser,
		authFactories: authFactories,
	}
}
