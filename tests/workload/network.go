// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
)

type TestNetwork interface {
	SubmitTxs(context.Context, string, []*chain.Transaction) error
	ConfirmTxs(context.Context, string, []*chain.Transaction) error
	URIs() []string
	WorkloadFactory() TxWorkloadFactory
}
