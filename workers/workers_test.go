// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workers

import (
	"context"
	"math/big"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/neilotoole/errgroup"
	"golang.org/x/sync/semaphore"
)

// Bench: go test -bench=. -benchtime=10000x -benchmem
//
// goos: darwin
// goarch: arm64
// pkg: github.com/ava-labs/hypersdk/workers
// BenchmarkWorker/10-10   	             10000	     10752 ns/op	    5921 B/op	      60 allocs/op
// BenchmarkWorker/50-10   	             10000	     36131 ns/op	   29603 B/op	     300 allocs/op
// BenchmarkWorker/100-10  	             10000	    107860 ns/op	   59203 B/op	     600 allocs/op
// BenchmarkWorker/500-10  	             10000	    415090 ns/op	  297972 B/op	    3244 allocs/op
// BenchmarkWorker/1000-10 	             10000	    644211 ns/op	  597990 B/op	    6744 allocs/op
//
// BenchmarkErrGroup/10-10 	             10000	      7276 ns/op	    6459 B/op	      71 allocs/op
// BenchmarkErrGroup/50-10 	             10000	     28234 ns/op	   30138 B/op	     311 allocs/op
// BenchmarkErrGroup/100-10         	   10000	     53299 ns/op	   59738 B/op	     611 allocs/op
// BenchmarkErrGroup/500-10         	   10000	    326090 ns/op	  298496 B/op	    3255 allocs/op
// BenchmarkErrGroup/1000-10        	   10000	    655961 ns/op	  598508 B/op	    6755 allocs/op
//
// BenchmarkNative/10-10            	   10000	      7032 ns/op	    6234 B/op	      63 allocs/op
// BenchmarkNative/50-10            	   10000	     33855 ns/op	   32877 B/op	     347 allocs/op
// BenchmarkNative/100-10           	   10000	     68023 ns/op	   65422 B/op	     687 allocs/op
// BenchmarkNative/500-10           	   10000	    360846 ns/op	  327939 B/op	    3656 allocs/op
// BenchmarkNative/1000-10          	   10000	    699992 ns/op	  656515 B/op	    7542 allocs/op
//
// BenchmarkSerial/10-10            	   10000	      5764 ns/op	    5760 B/op	      50 allocs/op
// BenchmarkSerial/50-10            	   10000	     28931 ns/op	   28800 B/op	     250 allocs/op
// BenchmarkSerial/100-10           	   10000	     57596 ns/op	   57600 B/op	     500 allocs/op
// BenchmarkSerial/500-10           	   10000	    290150 ns/op	  289952 B/op	    2744 allocs/op
// BenchmarkSerial/1000-10          	   10000	    580588 ns/op	  581955 B/op	    5744 allocs/op

func BenchmarkWorker(b *testing.B) {
	cores := runtime.NumCPU() - 1
	if cores == 0 {
		cores = 1
	}
	for _, size := range []int{10, 50, 100, 500, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			w := New(cores, 1_000)
			for n := 0; n < b.N; n++ {
				j, _ := w.NewJob(size)
				for i := 0; i < size; i++ {
					ti := i
					j.Go(func() error {
						bi := big.NewInt(int64(ti))
						hashing.ComputeHash256(bi.Bytes())
						return nil
					})
				}
				_ = j.Wait()
			}
		})
	}
}

func BenchmarkErrGroup(b *testing.B) {
	cores := runtime.NumCPU() - 1
	if cores == 0 {
		cores = 1
	}
	for _, size := range []int{10, 50, 100, 500, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			w, _ := errgroup.WithContextN(context.TODO(), cores, cores*4)
			for n := 0; n < b.N; n++ {
				for i := 0; i < size; i++ {
					j := i
					w.Go(func() error {
						bi := big.NewInt(int64(j))
						hashing.ComputeHash256(bi.Bytes())
						return nil
					})
				}
				_ = w.Wait()
			}
		})
	}
}

func BenchmarkNative(b *testing.B) {
	cores := runtime.NumCPU() - 1
	if cores == 0 {
		cores = 1
	}
	for _, size := range []int{10, 50, 100, 500, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			sm := semaphore.NewWeighted(int64(cores))
			for n := 0; n < b.N; n++ {
				wg := sync.WaitGroup{}
				for i := 0; i < size; i++ {
					_ = sm.Acquire(context.TODO(), 1)
					wg.Add(1)
					j := i
					go func() {
						bi := big.NewInt(int64(j))
						hashing.ComputeHash256(bi.Bytes())
						sm.Release(1)
						wg.Done()
					}()
				}
				wg.Wait()
			}
		})
	}
}

func BenchmarkSerial(b *testing.B) {
	for _, size := range []int{10, 50, 100, 500, 1000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				for i := 0; i < size; i++ {
					bi := big.NewInt(int64(i))
					hashing.ComputeHash256(bi.Bytes())
				}
			}
		})
	}
}
