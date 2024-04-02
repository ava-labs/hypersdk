package pebble

import (
	"crypto/rand"
	"fmt"
	"os"
	"testing"
)

const batchSize = 1_500_000

func randBytes() []byte {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func BenchmarkBatchInsertion(b *testing.B) {
	for _, sync := range []bool{false, true} {
		b.Run(fmt.Sprintf("sync=%t", sync), func(b *testing.B) {
			// Setup DB
			b.StopTimer()
			tdir := b.TempDir()
			cfg := NewDefaultConfig()
			cfg.Sync = sync
			db, _, err := New(tdir, cfg)
			if err != nil {
				b.Fatal(err)
			}

			// Setup keys
			keys := make([][]byte, batchSize)
			for i := 0; i < batchSize; i++ {
				keys[i] = randBytes()
			}

			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				batch := db.NewBatch()
				for j := 0; j < batchSize; j++ {
					if err := batch.Put(keys[j], randBytes()); err != nil {
						b.Fatal(err)
					}
				}
				if err := batch.Write(); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()

			if err := db.Close(); err != nil {
				b.Fatal(err)
			}
			if err := os.RemoveAll(tdir); err != nil {
				b.Fatal(err)
			}
		})
	}
}
