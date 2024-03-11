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
	cases   = []int{1, 16, 64, 128, 512, 2048, 4096, 16384}
	workers = []int{1, 2, 4, 8, 16, 24, 32}
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
	for _, sigs := range cases {
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
	for _, sigs := range cases {
		b.Run(fmt.Sprintf("sigs=%d", sigs), func(b *testing.B) {
			b.StopTimer()
			msg, pub, sig := generateSignature(b)
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				wg := sync.WaitGroup{}
				for j := 0; j < sigs; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ed25519.Verify(pub, msg, sig)
					}()
				}
				wg.Wait()
			}
		})
	}
}

func BenchmarkErrgroup(b *testing.B) {
	for _, w := range workers {
		for _, sigs := range cases {
			b.Run(fmt.Sprintf("sigs=%d w=%d", sigs, w), func(b *testing.B) {
				b.StopTimer()
				msg, pub, sig := generateSignature(b)
				b.StartTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					g, _ := errgroup.WithContext(context.TODO())
					g.SetLimit(w)
					for j := 0; j < sigs; j++ {
						g.Go(func() error {
							ed25519.Verify(pub, msg, sig)
							return nil
						})
					}
					g.Wait()
				}
			})
		}
	}
}

func BenchmarkOPool(b *testing.B) {
	for _, w := range workers {
		for _, sigs := range cases {
			b.Run(fmt.Sprintf("sigs=%d w=%d", sigs, w), func(b *testing.B) {
				b.StopTimer()
				msg, pub, sig := generateSignature(b)
				b.StartTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					op := New(w, sigs)
					for j := 0; j < sigs; j++ {
						op.Go(func() (func(), error) {
							ed25519.Verify(pub, msg, sig)
							return nil, nil
						})
					}
					op.Wait()
				}
			})
		}
	}
}
