// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"
)

var (
	// _              workload.TxWorkloadFactory  = (*workloadFactory)(nil)
	// _              workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
)

const (
	TxCheckInterval        = 100 * time.Millisecond
	fuel                   = 100_000_000
)


type workloadFactory struct {
	keys []*fixture.Ed25519TestKey
}

func NewWorkloadFactory(keys []*fixture.Ed25519TestKey) *workloadFactory {
	return &workloadFactory{keys: keys}
}

// func (f *workloadFactory) NewSizedTxWorkload(uri string, size int) (workload.TxWorkloadIterator, error) {
// 	cli := jsonrpc.NewJSONRPCClient(uri)
// 	lcli := vm.NewJSONRPCClient(uri)
// 	return &simpleTxWorkload{
// 		factory: f.keys[0].AuthFactory,
// 		cli:     cli,
// 		lcli:    lcli,
// 		size:    size,
// 	}, nil
// }

type simpleTxWorkload struct {
	factory *auth.ED25519Factory
	cli     *jsonrpc.JSONRPCClient
	lcli    *vm.JSONRPCClient
	count   int
	size    int

	// address we will be sending to
	counterAddess codec.Address
}

// func (g *simpleTxWorkload) Next() bool {
// 	return g.count < g.size
// }

// func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
// 	utils.Outf("{{green}}GENERATING:{{/}} %s\n", "callll increment")
	
// 	g.count++
// 	privKey, err := ed25519.GeneratePrivateKey()
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	pubKey := auth.NewED25519Address(privKey.PublicKey())
// 	parser, err := g.lcli.Parser(ctx)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	id := ids.GenerateTestID()
// 	deployAction := &actions.Deploy{
// 		ContractBytes: counterBytes,
// 		CreationData: id[:],
// 	}

// 	_, tx, _, err := g.cli.GenerateTransaction(
// 		ctx,
// 		parser,
// 		[]chain.Action{deployAction},
// 		g.factory,
// 	)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
// 		confirmIncrement(ctx, require, uri, tx.ID(), pubKey, 1)
// 	}, nil
// }

// func (g *simpleTxWorkload) GenerateTxWithAssertion(ctx context.Context) (*chain.Transaction, workload.TxAssertion, error) {
// 	utils.Outf("{{green}}GENERATING:{{/}} %s\n", "callll increment")
	
// 	g.count++
// 	privKey, err := ed25519.GeneratePrivateKey()
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	pubKey := auth.NewED25519Address(privKey.PublicKey())
// 	parser, err := g.lcli.Parser(ctx)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	// serialize args. We plan to inc the pubKey by 1
// 	args, err := actions.SerializeArgs(pubKey, 1)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	callAction := &actions.Call{
// 		ContractAddress: g.counterAddess,
// 		Value:           0,
// 		FunctionName:    "inc",
// 		Args:            args,
// 		Fuel:            fuel,
// 	}
// 	utils.Outf("{{green}}Simulating:{{/}} %s\n", "call increment")
// 	simResult, err := g.cli.SimulateActions(ctx, chain.Actions{callAction}, pubKey)
// 	utils.Outf("{{green}}Simulated:{{/}} %s\n", "call increment")
// 	if err != nil {
// 		utils.Outf("{{red}} ERROR HERE \n")
// 		return nil, nil, err
// 	}

// 	if len(simResult) != 1 {
// 		return nil, nil, fmt.Errorf("unexpected number of returned actions. One action expected, %d returned", len(simResult))
// 	}
// 	simRes := simResult[0]
// 	stateKeys := simRes.StateKeys
// 	callAction.SpecifiedStateKeys = make([]actions.StateKeyPermission, 0, len(stateKeys))
// 	for key, value := range simRes.StateKeys {
// 		utils.Outf("{{yellow}}StateKey %s\n", key)
// 		callAction.SpecifiedStateKeys = append(callAction.SpecifiedStateKeys, actions.StateKeyPermission{Key: key, Permission: value})
// 	}
// 	utils.Outf("{{blue}}StateKey %s\n", "i guess no statekeys")
// 	// utils.Outf("StateKeys: %v", callAction.SpecifiedStateKeys)

// 	_, tx, _, err := g.cli.GenerateTransaction(
// 		ctx,
// 		parser,
// 		[]chain.Action{callAction},
// 		g.factory,
// 	)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	return tx, func(ctx context.Context, require *require.Assertions, uri string) {
// 		confirmIncrement(ctx, require, uri, tx.ID(), pubKey, 1)
// 	}, nil
// }
