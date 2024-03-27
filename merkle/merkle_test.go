package merkle

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

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
	for _, sync := range []bool{false, true} {
		// Setup DB
		tdir := b.TempDir()
		cfg := pebble.NewDefaultConfig()
		cfg.Sync = sync
		pdb, _, err := pebble.New(tdir, cfg)
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

		// Run experiments
		keys := make([]string, 15_000_000)
		sizeArr := []int{100_000, 1_000_000, 10_000_000, 15_000_000}
		for s, size := range sizeArr {
			// Determine new item count
			last := 0
			if s > 0 {
				last = sizeArr[s-1]
			}

			// Add keys to DB
			ops := make(map[string]maybe.Maybe[[]byte], size-last)
			for j := last; j < size; j++ {
				keys[j] = string(randBytes())
				ops[keys[j]] = maybe.Some(randBytes())
			}
			view, err := db.NewView(context.TODO(), merkledb.ViewChanges{MapOps: ops})
			if err != nil {
				b.Fatal(err)
			}
			if err := view.CommitToDB(context.TODO()); err != nil {
				b.Fatal(err)
			}

			// Run through ops
			for _, keyOps := range []int{10_000, 25_000, 50_000, 75_000, 100_000, 250_000, 500_000, 1_000_000, 2_500_000, 5_000_000, 7_500_000, 10_000_000} {
				if keyOps > size {
					continue
				}
				b.Run(fmt.Sprintf("sync=%t_keys=%d_ops=%d", sync, size, keyOps), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						// Initialize sampler (ensure not just re-setting the same keys)
						s := sampler.NewUniform()
						s.Initialize(uint64(keyOps))

						// Generate new MapOps
						ops = make(map[string]maybe.Maybe[[]byte], keyOps)
						for j := 0; j < keyOps; j++ {
							idx, err := s.Next()
							if err != nil {
								b.Fatal(err)
							}
							ops[keys[idx]] = maybe.Some(randBytes())
						}

						// Create view, commit, get root
						viewStart := time.Now()
						view, err = db.NewView(context.TODO(), merkledb.ViewChanges{MapOps: ops})
						if err != nil {
							b.Fatal(err)
						}
						viewDur := time.Since(viewStart)
						commitStart := time.Now()
						if err := view.CommitToDB(context.TODO()); err != nil {
							b.Fatal(err)
						}
						b.Log("view creation", viewDur, "commit", time.Since(commitStart))
					}
				})
			}
		}

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
	}
}
