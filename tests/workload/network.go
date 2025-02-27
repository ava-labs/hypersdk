// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
)

var _ TestNetworkConfiguration = DefaultTestNetworkConfiguration{}

type TestNetwork interface {
	ConfirmTxs(context.Context, []*chain.Transaction) error
	GenerateTx(context.Context, []chain.Action, chain.AuthFactory) (*chain.Transaction, error)
	URIs() []string
	Configuration() TestNetworkConfiguration
}

// TestNetworkConfiguration provides network information required to run tests against
// a network.
//
// Implementations must be thread-safe.
type TestNetworkConfiguration interface {
	// Name returns the name of the network.
	Name() string
	// GenesisAndRuleFactory returns the factory to produce the genesis and rule factory of the network.
	// This interface requires the factory as opposed to the genesis or rule factory directly, because loading
	// the chain's rule factory requires the chainID and networkID, which are provided to the VM after
	// the chain is initialized.
	GenesisAndRuleFactory() genesis.GenesisAndRuleFactory
	// GenesisBytes provide the genesis bytes of the network. This is used if the test needs to initialize
	// the test network by issuing transactions to create the chain on the P-Chain.
	GenesisBytes() []byte
	// Parser returns the parser to parse the chain's Action/Auth definitions.
	Parser() chain.Parser
	// AuthFactories returns a slice of pre-funded [AuthFactory] instances to sign/generate transactions throughout
	// the test.
	AuthFactories() []chain.AuthFactory
}

// DefaultTestNetworkConfiguration struct is the common test configuration that a test framework would need to provide
// in order to deploy a network. A test would typically embed this as part of it's network configuration structure.
type DefaultTestNetworkConfiguration struct {
	name                  string
	genesisAndRuleFactory genesis.GenesisAndRuleFactory
	genesisBytes          []byte

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

func (d DefaultTestNetworkConfiguration) GenesisAndRuleFactory() genesis.GenesisAndRuleFactory {
	return d.genesisAndRuleFactory
}

func (d DefaultTestNetworkConfiguration) AuthFactories() []chain.AuthFactory {
	return d.authFactories
}

// NewDefaultTestNetworkConfiguration creates a new DefaultTestNetworkConfiguration object.
func NewDefaultTestNetworkConfiguration(
	name string,
	genesisAndRuleFactory genesis.GenesisAndRuleFactory,
	genesisBytes []byte,
	parser chain.Parser,
	authFactories []chain.AuthFactory,
) DefaultTestNetworkConfiguration {
	return DefaultTestNetworkConfiguration{
		name:                  name,
		genesisAndRuleFactory: genesisAndRuleFactory,
		genesisBytes:          genesisBytes,
		parser:                parser,
		authFactories:         authFactories,
	}
}
