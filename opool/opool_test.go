package opool

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
)

var (
	signatures = []int{1024, 4096, 16384, 32768, 65536, 131072}
	workers    = []int{2, 4, 8, 16, 24, 32}
	batches    = []int{1, 2, 4, 8, 16, 32, 64, 128}
)

func generateSignature(b *testing.B) ([]byte, ed25519.PublicKey, []byte) {
	msg := make([]byte, 128)
	_, err := rand.Read(msg)
	if err != nil {
		b.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		b.Fatal(err)
	}
	sig := ed25519.Sign(priv, msg)
	return msg, pub, sig
}

func BenchmarkBaseline(b *testing.B) {
	for _, sigs := range signatures {
		b.Run(fmt.Sprintf("sigs=%d", sigs), func(b *testing.B) {
			b.StopTimer()
			msg, pub, sig := generateSignature(b)
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for j := 0; j < sigs; j++ {
					ed25519.Verify(pub, msg, sig)
				}
			}
		})
	}
}

func BenchmarkNewGoroutines(b *testing.B) {
	for _, sigs := range signatures {
		for _, batch := range batches {
			if batch > sigs {
				continue
			}
			b.Run(fmt.Sprintf("sigs=%d batch=%d", sigs, batch), func(b *testing.B) {
				b.StopTimer()
				msg, pub, sig := generateSignature(b)
				b.StartTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					wg := sync.WaitGroup{}
					started := 0
					for started < sigs {
						run := min(batch, sigs-started)
						started += run
						wg.Add(1)
						go func() {
							defer wg.Done()

							for j := 0; j < run; j++ {
								ed25519.Verify(pub, msg, sig)
							}
						}()
					}
					wg.Wait()
				}
			})
		}
	}
}

func BenchmarkErrgroup(b *testing.B) {
	for _, sigs := range signatures {
		for _, w := range workers {
			for _, batch := range batches {
				if batch > sigs {
					continue
				}
				if w > sigs {
					continue
				}
				b.Run(fmt.Sprintf("sigs=%d w=%d batch=%d", sigs, w, batch), func(b *testing.B) {
					b.StopTimer()
					msg, pub, sig := generateSignature(b)
					b.StartTimer()
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						g, _ := errgroup.WithContext(context.TODO())
						g.SetLimit(w)
						started := 0
						for started < sigs {
							run := min(batch, sigs-started)
							started += run
							g.Go(func() error {
								for j := 0; j < run; j++ {
									ed25519.Verify(pub, msg, sig)
								}
								return nil
							})
						}
						g.Wait()
					}
				})
			}
		}
	}
}

func BenchmarkOPool(b *testing.B) {
	for _, sigs := range signatures {
		for _, w := range workers {
			for _, batch := range batches {
				if batch > sigs {
					continue
				}
				if w > sigs {
					continue
				}
				b.Run(fmt.Sprintf("sigs=%d w=%d batch=%d", sigs, w, batch), func(b *testing.B) {
					b.StopTimer()
					msg, pub, sig := generateSignature(b)
					b.StartTimer()
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						op := New(w, sigs)
						started := 0
						for started < sigs {
							run := min(batch, sigs-started)
							started += run
							op.Go(func() (func(), error) {
								for j := 0; j < run; j++ {
									ed25519.Verify(pub, msg, sig)
								}
								return nil, nil
							})
						}
						op.Wait()
					}
				})
			}
		}
	}
}
