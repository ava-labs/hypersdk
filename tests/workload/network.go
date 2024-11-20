// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

type TestNetwork interface {
	ConfirmTxs(context.Context, []*chain.Transaction) error
	GenerateTx(context.Context, []chain.Action, chain.AuthFactory) (*chain.Transaction, error)
	URIs() []string
	Configuration() TestNetworkConfiguration
	FundedAuthFactory() (chain.AuthFactory, error)
}

// TestNetworkConfiguration is an interface, implemented by VM-specific tests
// to store information regarding the test network prior to it's invocation, and
// retrieve it during execution. All implementations must be thread-safe.
type TestNetworkConfiguration interface {
	GenesisBytes() []byte
	Name() string
	Parser() chain.Parser
	PrivateKeys() []*auth.PrivateKey
}

// DefaultTestNetworkConfiguration struct is the common test configuration that a test framework would need to provide
// in order to deploy a network. A test would typically embed this as part of it's network configuration structure.
type DefaultTestNetworkConfiguration struct {
	genesisBytes []byte
	name         string
	parser       chain.Parser
	keys         []ed25519.PrivateKey
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

func (n DefaultTestNetworkConfiguration) PrivateKeys() []*auth.PrivateKey {
	keys := make([]*auth.PrivateKey, 0, len(n.keys))
	for _, key := range n.keys {
		keys = append(keys, auth.NewPrivateKeyFromED25519(key))
	}
	return keys
}

func (n *DefaultTestNetworkConfiguration) Keys() []ed25519.PrivateKey {
	return n.keys
}

// NewDefaultTestNetworkConfiguration creates a new DefaultTestNetworkConfiguration object.
func NewDefaultTestNetworkConfiguration(genesisBytes []byte, name string, parser chain.Parser, keys []ed25519.PrivateKey) DefaultTestNetworkConfiguration {
	return DefaultTestNetworkConfiguration{
		genesisBytes: genesisBytes,
		name:         name,
		parser:       parser,
		keys:         keys,
	}
}
