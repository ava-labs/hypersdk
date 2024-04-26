package merkle

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"
)

var (
	resRoot ids.ID
	resDB   merkledb.MerkleDB
	resErr  error
)

func BenchmarkMerkleTxRoot(b *testing.B) {
	for _, size := range []int{10, 100, 1000, 10000} {
		ctx := context.TODO()
		tracer := trace.Noop
		merkleItems := make([][]byte, 0, size)
		for i := 0; i < size; i++ {
			item := make([]byte, 32)
			_, err := rand.Read(item)
			require.NoError(b, err)
			merkleItems = append(merkleItems, item)
		}

		var root ids.ID
		var db merkledb.MerkleDB
		var err error

		defaultConfig := merkledb.Config{
			BranchFactor:              merkledb.BranchFactor16,
			HistoryLength:             100,
			IntermediateNodeCacheSize: units.MiB,
			ValueNodeCacheSize:        units.MiB,
			Tracer:                    tracer,
		}

		b.Run(strconv.Itoa(size), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				for i := 0; i < size; i++ {
					root, db, err = GenerateMerkleRoot(ctx, defaultConfig, merkleItems, false)
				}
			}
		})

		// avoid compiler optimizations to cancel out the bench
		resRoot = root
		resDB = db
		resErr = err
	}

	b.ReportAllocs()
}
