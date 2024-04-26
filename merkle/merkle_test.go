package merkle

import (
	"testing"

	"crypto/rand"
	"context"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

var resRoot ids.ID
var resDb merkledb.MerkleDB
var resErr error

func BenchmarkMerkleTxRoot(b *testing.B) {
	for _, size := range []int{10, 100, 1000, 10000} {
    ctx := context.TODO()
    tracer := trace.Noop
    merkleItems := make([][]byte, 0, size)
    for i := 0; i < size; i++ {
      item := make([]byte, 32)
      _, err := rand.Read(item)
      if err != nil {
        b.Fatal(err)
      }
      merkleItems = append(merkleItems, item)
    }

    var root ids.ID
    var db merkledb.MerkleDB
    var err error

		b.Run(strconv.Itoa(size), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				for i := 0; i < size; i++ {
          root, db, err = GenerateMerkleRoot(ctx, tracer, merkleItems, false)
				}
			}
		})

    // avoid compiler optimizations to cancel out the bench
    resRoot = root
    resDb = db
    resErr = err
	}

  b.ReportAllocs()
}
