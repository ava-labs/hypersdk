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

var _ workload.TxGenerator = (*DeployTransaction)(nil)

type DeployTransaction struct {
	numGenerated int
	maxGenerated int
	// the factory to generate the signature from
	from *auth.ED25519Factory
}

func (d *DeployTransaction) NewTxGenerator(maxToGenerate int) workload.TxGenerator {
	return &DeployTransaction{
		numGenerated: 0,
		maxGenerated: maxToGenerate,
		from: d.from,
	}
}

func (d *DeployTransaction) CanGenerate() bool {
	return d.numGenerated < d.maxGenerated
}

// generates a transaction deploying a contract to the VM. 
// Returns the transaction, the assertion to check the transaction, and an error if there was an issue generating the transaction
// shouldn't be creating a new jsonrpc client every time, but keeping for simplicity for now
func (d *DeployTransaction) GenerateTx(ctx context.Context, uri string) (*chain.Transaction, workload.TxAssertion, error) {
	defer func() { d.numGenerated++ }()

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