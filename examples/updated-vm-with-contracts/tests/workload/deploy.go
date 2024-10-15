package workload

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/actions"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/vm"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/stretchr/testify/require"
)

type DeployTransaction struct {
	// the factory to generate the signature from
	from *auth.ED25519Factory
}

// generates a transaction deploying a contract to the VM. 
// Returns the transaction, the assertion to check the transaction, and an error if there was an issue generating the transaction
func (d *DeployTransaction) GenerateTransaction(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	// create a new jsonrpc client
	cli := jsonrpc.NewJSONRPCClient(uri)
	vmCli := vm.NewJSONRPCClient(uri)
	parser, err := vmCli.Parser(ctx)
	if err != nil {
		return nil, nil, err
	} 

	// create deploy action
	deployAction := &actions.Deploy{
		ContractBytes: []byte("contract code"),
		CreationData: []byte("creation data"),
	}
	
	// create the transaction
	_, tx, _, err := cli.GenerateTransaction(ctx, parser, []chain.Action{deployAction}, d.from)
	if err != nil {
		return nil, nil, err
	}

	return tx, 
		func(ctx context.Context, require *require.Assertions, uri string) {
			confirmDeploy(ctx, require, uri, tx.ID())
		}, nil
}

func confirmDeploy(ctx context.Context,  require *require.Assertions, uri string, txID ids.ID) error {
	// create a new jsonrpc client
	indexerCli := indexer.NewClient(uri)
	// wait for the transaction to be accepted
	success, _, err := indexerCli.WaitForTransaction(ctx, TxCheckInterval, txID)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("transaction failed")
	}
	return nil
}