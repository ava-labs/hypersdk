package chain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/trace"
	gomock "github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func setupProcessor(b *testing.B) (*Processor, merkledb.TrieView){
	b.Helper()
	require := require.New(b)
	
	chainId := ids.GenerateTestID()
	rawStateDB := memdb.New()
	tracer, err := trace.New(&trace.Config{Enabled: false})
	require.NoError(err)
	merkleRegistry := prometheus.NewRegistry()
	stateDB, err := merkledb.New(context.TODO(), rawStateDB, merkledb.Config{
		HistoryLength: 256,
		NodeCacheSize: 65_536,
		Reg:           merkleRegistry,
		Tracer:        tracer,
	})
	require.NoError(err)

	trie, err := stateDB.NewView()
	require.NoError(err)

	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	r := NewMockRules(ctrl)
	vm := NewMockVM(ctrl)
	sm := NewMockStateManager(ctrl)

	nextTime := time.Now().Unix()

	parent := &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			Tmstmp: 1,
			Prnt:   ids.Empty,
			Hght:   0, 
			BlockCost: 1000,
		},
		st: choices.Processing,
	}

	// rules
	r.EXPECT().GetWindowTargetBlocks().AnyTimes()
	r.EXPECT().GetWindowTargetUnits().AnyTimes()
	r.EXPECT().GetUnitPriceChangeDenominator().AnyTimes()
	r.EXPECT().GetMinUnitPrice().AnyTimes()
	r.EXPECT().GetBlockCostChangeDenominator().AnyTimes()
	r.EXPECT().GetMinBlockCost().AnyTimes()

	// sm
	sm.EXPECT().OutgoingWarpKey(chainId).Return(prefixTestTxKey(chainId)).AnyTimes()

	// vm
	vm.EXPECT().StateManager().Return(sm).AnyTimes()

	ectx, err := GenerateExecutionContext(context.TODO(), chainId, nextTime, parent, tracer, r)
	require.NoError(err)

	blk := NewBlock(ectx, vm, parent, nextTime)
	blk.Txs = []*Transaction{createTestTx(b, 10, nextTime, chainId)}
	return NewProcessor(tracer, blk), trie
}

func BenchmarkProcessorPrefetch(b *testing.B) {
		b.ResetTimer()
		b.Run(fmt.Sprintf("processor_%d_keys", 10), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				p, db := setupProcessor(b)
				b.StartTimer()
				p.Prefetch(context.Background(), db)
				// clear channel
				for range p.readyTxs {}
			}
		})
}

func createTestTx(b *testing.B, keySize int, tmstp int64, chainId ids.ID)*Transaction {
	b.Helper()
	ctrl := gomock.NewController(b)
	act := NewMockAction(ctrl)
	auth := NewMockAuth(ctrl)

	keys := make([][]byte, 0, keySize)
	for i := 0; i < keySize/2 ; i++ {
		keys = append(keys, prefixTestTxKey(ids.GenerateTestID()))
	}

	auth.EXPECT().StateKeys().Return(keys).AnyTimes()
	act.EXPECT().StateKeys(auth, chainId).Return(keys).AnyTimes()
	tx := &Transaction{
			Base: &Base{
				Timestamp: tmstp,
				ChainID: chainId,
				UnitPrice:  10,
			},
			Action: act,
			Auth: auth,
			id: chainId,
		}

	return tx
}

const (
	txPrefix = 0x0
	iDLen    = 32
)

func prefixTestTxKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+iDLen)
	k[0] = txPrefix
	copy(k[1:], id[:])
	return
}