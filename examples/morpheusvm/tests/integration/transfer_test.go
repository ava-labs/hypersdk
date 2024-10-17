// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/integration"
	"github.com/ava-labs/hypersdk/tests/workload"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = integration.RegisterTest("Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn workload.TestNetwork) error {
	require := require.New(t)

	uri := tn.URIs()[0]
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	aother := auth.NewED25519Address(other.PublicKey())

	lcli := vm.NewJSONRPCClient(uri)
	parser, err := lcli.Parser(context.Background())
	require.NoError(err)

	spendingKey, err := tn.WorkloadFactory().GetSpendingKey()
	require.NoError(err)

	cli := jsonrpc.NewJSONRPCClient(uri)
	_, tx, _, err := cli.GenerateTransaction(
		context.Background(),
		parser,
		[]chain.Action{&actions.Transfer{
			To:    aother,
			Value: 1,
		}},
		auth.NewED25519Factory(ed25519.PrivateKey(spendingKey.Bytes)),
	)

	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.SubmitTxs(timeoutCtx, uri, []*chain.Transaction{tx}))
	require.NoError(tn.ConfirmTxs(timeoutCtx, uri, []*chain.Transaction{tx}))
	return nil
})