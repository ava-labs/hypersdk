package merkle

import (
	"context"
	"crypto/rand"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
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
	// Setup DB
	tdir := b.TempDir()
	cfg := pebble.NewDefaultConfig()
	cfg.Sync = true
	pdb, _, err := pebble.New(tdir, cfg)
	if err != nil {
		b.Fatal(err)
	}
	db, err := merkledb.New(context.TODO(), pdb, merkledb.Config{
		BranchFactor:                merkledb.BranchFactor16,
		RootGenConcurrency:          1,
		HistoryLength:               0,
		ValueNodeCacheSize:          3 * units.GiB,
		IntermediateNodeCacheSize:   3 * units.GiB,
		IntermediateWriteBufferSize: 2 * units.GiB,
		IntermediateWriteBatchSize:  128 * units.MiB,
		Tracer:                      trace.Noop,
	})
	if err != nil {
		b.Fatal(err)
	}

	// Run experiments
	keys := make([]string, 15_000_000)
	// Add keys to DB
	ops := make(map[string]maybe.Maybe[[]byte], 10_000_000)
	for j := 0; j < 10_000_000; j++ {
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
