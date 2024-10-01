package spam

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/oexpirer"
)

// TODO: we should NEVER use globals, remove this
var (
	ctx            = context.Background()
	maxConcurrency = runtime.NumCPU()

	validityWindow     int64
	chainID            ids.ID
	uris               map[string]string
	maxPendingMessages int
	parser             chain.Parser

	issuerWg sync.WaitGroup

	l                sync.Mutex
	confirmationTime uint64 // reset each second
	delayTime        int64  // reset each second
	delayCount       uint64 // reset each second
	successTxs       uint64
	expiredTxs       uint64
	droppedTxs       uint64
	failedTxs        uint64
	invalidTxs       uint64
	totalTxs         uint64
	activeAccounts   = set.NewSet[codec.Address](16_384)

	pending = oexpirer.New[*txWrapper](16_384)

	issuedTxs     atomic.Int64
	sent          atomic.Int64
	sentBytes     atomic.Int64
	received      atomic.Int64
	receivedBytes atomic.Int64
)
