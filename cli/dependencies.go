package cli

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type Controller interface {
	DatabasePath() string
	Symbol() string
	Decimals() uint8
	Address(codec.Address) string
	ParseAddress(string) (codec.Address, error)
}

type SpamHelper interface {
	// CreateAccount generates a new account and returns the [PrivateKey].
	//
	// The spammer tracks all created accounts and orchestrates the return of funds
	// sent to any created accounts on shutdown. If the spammer exits ungracefully,
	// any funds sent to created accounts will be lost unless they are persisted by
	// the [SpamHelper] implementation.
	CreateAccount() (*PrivateKey, error)
	// GetFactory returns the [chain.AuthFactory] for a given private key.
	//
	// A [chain.AuthFactory] signs transactions and provides a unit estimate
	// for using a given private key (needed to estimate fees for a transaction).
	GetFactory(pk *PrivateKey) (chain.AuthFactory, error)

	// CreateClient instructs the [SpamHelper] to create and persist a VM-specific
	// JSONRPC client.
	//
	// This client is used to retrieve the [chain.Parser] and the balance
	// of arbitrary addresses.
	//
	// TODO: consider making these functions part of the required JSONRPC
	// interface for the HyperSDK.
	CreateClient(uri string, networkID uint32, chainID ids.ID) error
	GetParser(ctx context.Context) (chain.Parser, error)
	LookupBalance(choice int, address string) (uint64, error)

	// GetTransfer returns a list of actions that sends [amount] to a given [address].
	//
	// Memo is used to ensure that each transaction is unique (even if between the same
	// sender and receiver for the same amount).
	GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action
}
