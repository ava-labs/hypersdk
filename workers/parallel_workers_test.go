// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workers

import (
	"context"
	"errors"
	"math/big"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/neilotoole/errgroup"
	"github.com/stretchr/testify/require"
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
			w := NewParallel(cores, 1_000)
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

func TestJobWait(t *testing.T) {
	require := require.New(t)
	job := ParallelJob{
		tasks:     make(chan func() error),
		completed: make(chan struct{}),
		result:    make(chan error),
	}
	testError := errors.New("TestError")
	go func() {
		job.result <- testError
	}()
	returned := job.Wait()
	require.ErrorIs(testError, returned, "Incorrect error returned.")
}

func TestJobGo(t *testing.T) {
	require := require.New(t)
	job := ParallelJob{
		tasks:     make(chan func() error, 2),
		completed: make(chan struct{}),
		result:    make(chan error),
	}
	n := 1
	testError := errors.New("TestError")
	for n <= 2 {
		job.Go(func() error {
			if n == 1 {
				return nil
			} else {
				return testError
			}
		})
		n += 1
	}
	n = 1
	// Ensure corret errors
	for n <= 2 {
		response := <-job.tasks
		if n == 1 {
			require.NoError(response(), "Incorrect error message.")
		} else {
			require.ErrorIs(testError, response(), "Incorrect error message.")
		}
		n += 1
	}
}

func TestStopWorker(t *testing.T) {
	require := require.New(t)
	w := NewParallel(10, 100).(*ParallelWorkers)
	require.False(w.shouldShutdown, "Shutdown val incorrectly initialized.")
	w.Stop()
	require.Empty(w.queue, "Worker queue not empty")
	require.True(w.shouldShutdown, "Shutdown val not set.")
}

func TestWorker(t *testing.T) {
	require := require.New(t)
	w := NewParallel(10, 10000).(*ParallelWorkers)
	// Require workers was created properly
	require.Equal(10, w.count, "Count not set correctly")
	require.Empty(w.queue, "Worker queue not empty")
	require.Empty(w.tasks, "Worker tasks not empty")
	// Shutdown fields
	require.Empty(w.ackShutdown, "Worker ackShutdown not empty")
	require.Empty(w.stopWorkers, "Worker stopWorkers not empty")
	require.Empty(w.stoppedWorkers, "Worker stoppedWorkers not empty")
	// Value updated by the workers
	var valLock sync.Mutex
	val := 0
	jobs := []Job{}
	for i := 0; i < 1000; i++ {
		// Create a new job
		job, err := w.NewJob(10)
		require.NoError(err)
		for j := 0; j < 10; j++ {
			job.Go(func() error {
				valLock.Lock()
				defer valLock.Unlock()
				val += 1
				return nil
			})
		}
		jobs = append(jobs, job)
		job.Done(nil)
	}
	for _, j := range jobs {
		require.NoError(j.Wait(), "Error waiting on job.")
	}
	w.Stop()
	// Jobs ran correctly
	require.Equal(10000, val, "Value not updated correctly")
}

func TestWorkerStop(t *testing.T) {
	require := require.New(t)
	w := NewParallel(5, 100)
	var valLock sync.Mutex
	val := 0
	// Create a new job
	job, err := w.NewJob(5)
	require.NoError(err)
	for j := 0; j < 5; j++ {
		job.Go(func() error {
			time.Sleep(time.Second * 1)
			valLock.Lock()
			defer valLock.Unlock()
			val += 1
			return nil
		})
	}
	job.Done(nil)
	valLock.Lock()
	require.Equal(0, val, "Value not updated correctly")
	valLock.Unlock()
	require.NoError(job.Wait(), "Error waiting on job.")
	// Trigger stop, 5 jobs should still be running
	w.Stop()
	// Check that processes completed
	require.Equal(5, val, "Value not updated correctly")
	// Make sure new jobs cannot be created
	_, err = w.NewJob(5)
	require.ErrorIs(ErrShutdown, err, "Incorrect error thrown from NewJob.")
}

func TestNewJobShutdown(t *testing.T) {
	require := require.New(t)
	w := NewParallel(2, 10).(*ParallelWorkers)
	w.shouldShutdown = true
	job, err := w.NewJob(10)
	require.ErrorIs(ErrShutdown, err, "NewJob returned no error")
	require.Nil(job, "NewJob returned a not nil job pointer.")
}
