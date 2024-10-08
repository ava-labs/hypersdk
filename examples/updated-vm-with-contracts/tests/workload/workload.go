package workload

import (
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/vm"
	"github.com/ava-labs/hypersdk/tests/workload"
)

const (
	initialBalance  uint64 = 10_000_000_000_000
	txCheckInterval        = 100 * time.Millisecond
)

var (
	_              workload.TxWorkloadFactory  = (*factory)(nil)
	_              workload.TxWorkloadIterator = (*simpleTxWorkload)(nil)
)

type simpleTxWorkload struct {
	factory *auth.ED25519Factory
	cli     *jsonrpc.JSONRPCClient
	lcli    *vm.JSONRPCClient
	count   int
	size    int
}

func (w *simpleTxWorkload) Next() workload.TxWorkload {
	panic("implement me")
}
