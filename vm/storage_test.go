package vm

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/sampler"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/pebble"
)

func randBytes() []byte {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func BenchmarkMerkleDB(b *testing.B) {
	for _, items := range []int{1_000, 10_000, 100_000, 1_000_000, 10_000_000} {
		for _, actions := range []int{100, 1_000, 10_000, 100_000, 1_000_000} {
			if actions > items {
				continue
			}
			b.Run(fmt.Sprintf("items=%d_actions=%d", items, actions), func(b *testing.B) {
				b.StopTimer()

				// Setup DB
				tdir := b.TempDir()
				pdb, _, err := pebble.New(tdir, pebble.NewDefaultConfig())
				if err != nil {
					b.Fatal(err)
				}
				db, err := merkledb.New(context.TODO(), pdb, merkledb.Config{
					BranchFactor:                merkledb.BranchFactor16,
					RootGenConcurrency:          uint(runtime.NumCPU()),
					HistoryLength:               0,
					ValueNodeCacheSize:          128 * units.MiB,
					IntermediateWriteBufferSize: 128 * units.MiB,
					IntermediateWriteBatchSize:  128 * units.MiB,
					Tracer:                      trace.Noop,
				})
				if err != nil {
					b.Fatal(err)
				}

				// Add keys to DB
				keys := make([][]byte, items)
				ops := make(map[string]maybe.Maybe[[]byte], items)
				for j := 0; j < items; j++ {
					k := randBytes()
					if j < actions {
						keys[j] = k
					}
					ops[string(k)] = maybe.Some(randBytes())
				}
				view, err := db.NewView(context.TODO(), merkledb.ViewChanges{MapOps: ops})
				if err != nil {
					b.Fatal(err)
				}
				if err := view.CommitToDB(context.TODO()); err != nil {
					b.Fatal(err)
				}

				b.StartTimer()
				for i := 0; i < b.N; i++ {
					// Initialize sampler (ensure not just re-setting the same keys)
					s := sampler.NewUniform()
					s.Initialize(uint64(items))

					// Generate new MapOps
					ops = make(map[string]maybe.Maybe[[]byte], actions)
					for j := 0; j < actions; j++ {
						idx, err := s.Next()
						if err != nil {
							b.Fatal(err)
						}
						ops[string(keys[idx])] = maybe.Some(randBytes())
					}
					// Create view, commit, get root
					view, err = db.NewView(context.TODO(), merkledb.ViewChanges{MapOps: ops})
					if err != nil {
						b.Fatal(err)
					}
					if err := view.CommitToDB(context.TODO()); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()

				// Remove created files to ensure we don't run into any error logs
				if err := db.Close(); err != nil {
					b.Fatal(err)
				}
				if err := pdb.Close(); err != nil {
					b.Fatal(err)
				}
				if err := os.RemoveAll(tdir); err != nil {
					b.Fatal(err)
				}
			})
		}
	}
}
