package common

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/rpc"
)

// RpcClient is an interface of common rpc client functions
type RpcClient interface {
	Parser(ctx context.Context) (chain.Parser, error)
	WaitForTransaction(ctx context.Context, txID ids.ID) (bool, uint64, error)
}

// NodeInstance is a struct that holds information about a node
type NodeInstance struct {
	NodeID ids.NodeID
	Uri    string
	Cli    *rpc.JSONRPCClient
	VmCli  interface{}
}
