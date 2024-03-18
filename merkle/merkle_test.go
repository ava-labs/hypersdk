package merkle

import (
	"encoding/binary"
	"testing"

	"context"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

var res_root ids.ID
var res_db merkledb.MerkleDB
var res_err error

func BenchmarkMerkleTxRoot(b *testing.B) {
	for _, size := range []int{10, 100, 1000, 10000} {
    ctx := context.TODO()
    tracer := trace.Noop
    merkleItems := make([][]byte, 0, size)
    for i := 0; i < size; i++ {
      as_bytes := make([]byte, 32)
      binary.BigEndian.PutUint32(as_bytes, uint32(i))
      merkleItems = append(merkleItems, as_bytes)
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
    res_root = root
    res_db = db
    res_err = err
	}
}
