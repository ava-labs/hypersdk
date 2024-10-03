package loadgen

import (
	"context"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type SpamHelper interface {
	// CreateAccount generates a new account and returns the [PrivateKey].
	//
	// The spammer tracks all created accounts and orchestrates the return of funds
	// sent to any created accounts on shutdown. If the spammer exits ungracefully,
	// any funds sent to created accounts will be lost unless they are persisted by
	// the [SpamHelper] implementation.
	CreateAccount() (*auth.PrivateKey, error)

	// CreateClient instructs the [SpamHelper] to create and persist a VM-specific
	// JSONRPC client.
	//
	// This client is used to retrieve the [chain.Parser] and the balance
	// of arbitrary addresses.
	//
	// TODO: consider making these functions part of the required JSONRPC
	// interface for the HyperSDK.
	CreateClient(uri string) error
	GetParser(ctx context.Context) (chain.Parser, error)
	LookupBalance(choice int, address codec.Address) (uint64, error)

	// GetTransfer returns a list of actions that sends [amount] to a given [address].
	//
	// Memo is used to ensure that each transaction is unique (even if between the same
	// sender and receiver for the same amount).
	GetTransfer(address codec.Address, amount uint64, memo []byte) []chain.Action
}
