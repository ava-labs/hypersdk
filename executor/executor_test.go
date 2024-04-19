// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state"
)

const (
	numIterations   = 10 // Run several times to catch non-determinism
	maxDependencies = 100_000_000
)

func generateNumbers(start int) []int {
	array := make([]int, 9999)
	for i := 0; i < 9999; i++ {
		array[i] = i + start
	}
	return array
}

func TestExecutorNoConflicts(t *testing.T) {
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 100)
		e         = New(100, 4, maxDependencies, nil)
		canWait   = make(chan struct{})
	)

	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			l.Lock()
			completed = append(completed, ti)
			if len(completed) == 100 {
				close(canWait)
			}
			l.Unlock()
			return nil
		})
	}
	<-canWait
	require.NoError(e.Wait()) // no task running
	require.Len(completed, 100)
}

func TestExecutorNoConflictsSlow(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require   = require.New(t)
			l         sync.Mutex
			completed = make([]int, 0, 100)
			e         = New(100, 4, maxDependencies, nil)
			slow      = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
			}
			ti := i
			e.Run(s, func() error {
				if ti == 0 {
					<-slow
				}
				l.Lock()
				completed = append(completed, ti)
				if len(completed) == 99 {
					close(slow)
				}
				l.Unlock()
				return nil
			})
		}
		require.NoError(e.Wait()) // existing task is running
		require.Len(completed, 100)
		require.Equal(0, completed[99])
	}
}

func TestExecutorSimpleConflict(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, maxDependencies, nil)
		slow        = make(chan struct{})
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		if i%10 == 0 {
			s.Add(conflictKey, state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			if ti == 0 {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			if len(completed) == 90 {
				close(slow)
			}
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	require.Equal([]int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90}, completed[90:])
}

func TestExecutorMultiConflict(t *testing.T) {
	var (
		require      = require.New(t)
		conflictKey  = ids.GenerateTestID().String()
		conflictKey2 = ids.GenerateTestID().String()
		l            sync.Mutex
		completed    = make([]int, 0, 100)
		e            = New(100, 4, maxDependencies, nil)
		slow1        = make(chan struct{})
		slow2        = make(chan struct{})
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		if i%10 == 0 {
			s.Add(conflictKey, state.Read|state.Write)
		}
		if i == 15 || i == 20 {
			s.Add(conflictKey2, state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			if ti == 0 {
				<-slow1
			}
			if ti == 15 {
				<-slow2
			}

			l.Lock()
			completed = append(completed, ti)
			if len(completed) == 89 {
				close(slow1)
			}
			if len(completed) == 91 {
				close(slow2)
			}
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	require.Equal([]int{0, 10, 15, 20, 30, 40, 50, 60, 70, 80, 90}, completed[89:])
}

func TestEarlyExit(t *testing.T) {
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 500)
		e         = New(500, 4, maxDependencies, nil)
		terr      = errors.New("uh oh")
	)
	for i := 0; i < 500; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			if ti == 200 {
				return terr
			}
			return nil
		})
	}
	require.ErrorIs(e.Wait(), terr) // no task running
	require.Less(len(completed), 500)
}

func TestStop(t *testing.T) {
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 500)
		e         = New(500, 4, maxDependencies, nil)
	)
	for i := 0; i < 500; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			if ti == 200 {
				e.Stop()
			}
			return nil
		})
	}
	require.ErrorIs(e.Wait(), ErrStopped) // no task running
	require.Less(len(completed), 500)
}

// W->W->W->...
func TestManyWrites(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			answer      = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			answer = append(answer, i)
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				// add a bunch of different, non-overlapping keys to txn
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			// block on every txn to mimic sequential writes
			s.Add(conflictKey, state.Write)
			ti := i
			e.Run(s, func() error {
				// delay first txn
				if ti == 0 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		require.Equal(answer, completed)
	}
}

// R->R->R->...
func TestManyReads(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Read)
			}
			// mimic concurrent reading for all txns
			s.Add(conflictKey, state.Read)
			ti := i
			e.Run(s, func() error {
				// add some delays for the first 5 even numbers
				if ti < 10 && ti%2 == 0 {
					<-slow
				}
				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		for i := 0; i < 5; i++ {
			slow <- struct{}{}
		}
		close(slow)
		require.NoError(e.Wait())
		// 0..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

// W->R->R->...
func TestWriteThenRead(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			if i == 0 {
				s.Add(conflictKey, state.Write)
			} else {
				// add concurrent reading only after the first Write
				s.Add(conflictKey, state.Read)
			}
			ti := i
			e.Run(s, func() error {
				// add some delay to the first Write to enforce
				// all Reads after to wait until the first Write is done
				if ti == 0 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		require.Equal(0, completed[0]) // Write first to execute
		// 1..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

// R->R->W...
func TestReadThenWrite(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			if i == 10 {
				// add a Write after some concurrent Reads
				s.Add(conflictKey, state.Write)
			} else {
				s.Add(conflictKey, state.Read)
			}
			ti := i
			e.Run(s, func() error {
				// on the last Read before the first Write, add a delay
				// so that everything after (W and R) needs to wait until
				// this one is completed
				if ti == 9 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		// 0..9 are ran in parallel, so non-deterministic
		require.Equal(10, completed[10]) // First write to execute
		// 11..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

// W->R->R->...W->R->R->...
func TestWriteThenReadRepeated(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			if i == 0 || i == 49 {
				s.Add(conflictKey, state.Write)
			} else {
				s.Add(conflictKey, state.Read)
			}
			ti := i
			e.Run(s, func() error {
				if ti == 0 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		require.Equal(0, completed[0]) // First write to execute
		// 1..48 are ran in parallel, so non-deterministic
		require.Equal(49, completed[49]) // Second write to execute
		// 50..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

// R->R->W->R->W->R->R...
func TestReadThenWriteRepeated(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require     = require.New(t)
			conflictKey = ids.GenerateTestID().String()
			l           sync.Mutex
			completed   = make([]int, 0, 100)
			e           = New(100, 4, maxDependencies, nil)
			slow        = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			if i == 10 || i == 12 {
				s.Add(conflictKey, state.Write)
			} else {
				s.Add(conflictKey, state.Read)
			}
			ti := i
			e.Run(s, func() error {
				if ti == 10 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		// 0..9 are ran in parallel, so non-deterministic
		require.Equal(10, completed[10])
		require.Equal(11, completed[11])
		require.Equal(12, completed[12])
		// 13..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

func TestTwoConflictKeys(t *testing.T) {
	for j := 0; j < numIterations; j++ {
		var (
			require      = require.New(t)
			conflictKey1 = ids.GenerateTestID().String()
			conflictKey2 = ids.GenerateTestID().String()
			l            sync.Mutex
			completed    = make([]int, 0, 100)
			e            = New(100, 4, maxDependencies, nil)
			slow         = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			s.Add(conflictKey1, state.Read)
			s.Add(conflictKey2, state.Write)
			ti := i
			e.Run(s, func() error {
				if ti == 10 {
					<-slow
				}

				l.Lock()
				completed = append(completed, ti)
				l.Unlock()
				return nil
			})
		}
		close(slow)
		require.NoError(e.Wait())
		require.Equal(0, completed[0])
		require.Equal(1, completed[1])
		// 2..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}

func TestLargeConcurrentRead(t *testing.T) {
	rand.Seed(1)

	var (
		require   = require.New(t)
		numKeys   = 10
		numTxs    = 100_000
		completed = make([]int, 0, numTxs)
		e         = New(numTxs, 4, maxDependencies, nil)

		numBlocking = 1000
		blocking    = set.Set[int]{}

		numConflicts = 100
		conflictKeys = make([]string, 0, numConflicts)

		l    sync.Mutex
		slow = make(chan struct{})
	)

	// randomly wait on certain txs
	for blocking.Len() < numBlocking {
		blocking.Add(rand.Intn(numTxs)) //nolint:gosec
	}

	// make the conflict keys
	for k := 0; k < numConflicts; k++ {
		conflictKeys = append(conflictKeys, ids.GenerateTestID().String())
	}

	// Generate txs
	for i := 0; i < numTxs; i++ {
		// mix of unique and conflict keys in each tx
		s := make(state.Keys, (numKeys + 1))

		// random size of conflict keys to add
		setSize := max(1, rand.Intn(6))                   //nolint:gosec
		randomConflictingKeys := set.NewSet[int](setSize) // indices of [conflictKeys]
		for randomConflictingKeys.Len() < setSize {
			randomConflictingKeys.Add(rand.Intn(numConflicts)) //nolint:gosec
		}

		// add the random keys to tx
		for k := range randomConflictingKeys {
			s.Add(conflictKeys[k], state.Read)
		}

		// fill in rest with unique keys that are Reads
		remaining := numKeys - setSize
		for j := 0; j < remaining; j++ {
			s.Add(ids.GenerateTestID().String(), state.Read)
		}

		// pass into executor
		ti := i
		e.Run(s, func() error {
			if blocking.Contains(ti) {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	for i := 0; i < numBlocking; i++ {
		slow <- struct{}{}
	}
	close(slow)
	require.NoError(e.Wait())
	// concurrent Reads is non-deterministic
	require.Len(completed, numTxs)
}

func TestLargeSequentialWrites(t *testing.T) {
	rand.Seed(2)

	var (
		require   = require.New(t)
		numKeys   = 10
		numTxs    = 100_000
		completed = make([]int, 0, numTxs)
		e         = New(numTxs, 4, maxDependencies, nil)

		numBlocking = 1000
		blocking    = set.Set[int]{}

		numConflicts = 100
		conflictKeys = make([]string, 0, numConflicts)

		l    sync.Mutex
		slow = make(chan struct{})
	)

	// randomly wait on certain txs
	for blocking.Len() < numBlocking {
		blocking.Add(rand.Intn(numTxs)) //nolint:gosec
	}

	// make the conflict keys
	for k := 0; k < numConflicts; k++ {
		conflictKeys = append(conflictKeys, ids.GenerateTestID().String())
	}

	// Generate txs
	for i := 0; i < numTxs; i++ {
		// mix of unique and conflict keys in each tx
		s := make(state.Keys, (numKeys + 1))

		// random size of conflict keys to add
		setSize := max(1, rand.Intn(6))                   //nolint:gosec
		randomConflictingKeys := set.NewSet[int](setSize) // indices of [conflictKeys]
		for randomConflictingKeys.Len() < setSize {
			randomConflictingKeys.Add(rand.Intn(numConflicts)) //nolint:gosec
		}

		// add the random keys to tx
		for k := range randomConflictingKeys {
			s.Add(conflictKeys[k], state.Write)
		}

		// fill in rest with unique keys that are Writes
		remaining := numKeys - setSize
		for j := 0; j < remaining; j++ {
			s.Add(ids.GenerateTestID().String(), state.Write)
		}

		// pass into executor
		ti := i
		e.Run(s, func() error {
			if blocking.Contains(ti) {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	for i := 0; i < numBlocking; i++ {
		slow <- struct{}{}
	}
	close(slow)
	require.NoError(e.Wait())
	require.Len(completed, numTxs)
}

// This test adds all conflicting keys but tests over
// a large tx count, which is important to see if
// things break or not.
func TestLargeReadsThenWrites(t *testing.T) {
	rand.Seed(3)

	var (
		require   = require.New(t)
		numKeys   = 5
		numTxs    = 100_000
		completed = make([]int, 0, numTxs)
		answers   = make([][]int, 5)
		e         = New(numTxs, 4, maxDependencies, nil)

		numBlocking  = 10000
		blocking     = set.Set[int]{}
		conflictKeys = make([]string, 0, numKeys)

		l    sync.Mutex
		slow = make(chan struct{})
	)

	// make [answers] matrix
	answers[0] = generateNumbers(10000)
	answers[1] = generateNumbers(30000)
	answers[2] = generateNumbers(50000)
	answers[3] = generateNumbers(70000)
	answers[4] = generateNumbers(90000)

	// randomly wait on certain txs
	for blocking.Len() < numBlocking {
		blocking.Add(rand.Intn(numTxs)) //nolint:gosec
	}

	// make the conflict keys
	for k := 0; k < numKeys; k++ {
		conflictKeys = append(conflictKeys, ids.GenerateTestID().String())
	}

	// Generate txs
	for i := 0; i < numTxs; i++ {
		s := make(state.Keys, (numKeys + 1))
		for j := 0; j < numKeys; j++ {
			mode := (i / 10000) % 2
			switch mode {
			case 0:
				s.Add(conflictKeys[j], state.Read)
			case 1:
				s.Add(conflictKeys[j], state.Write)
			}
		}

		// pass into executor
		ti := i
		e.Run(s, func() error {
			if blocking.Contains(ti) {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	for i := 0; i < numBlocking; i++ {
		slow <- struct{}{}
	}
	close(slow)
	require.NoError(e.Wait())
	require.Len(completed, numTxs)
	require.Equal(answers[0], completed[10000:19999])
	require.Equal(answers[1], completed[30000:39999])
	require.Equal(answers[2], completed[50000:59999])
	require.Equal(answers[3], completed[70000:79999])
	require.Equal(answers[4], completed[90000:99999])
}

// This test adds all conflicting keys but tests over
// a large tx count, which is important to see if
// things break or not.
func TestLargeWritesThenReads(t *testing.T) {
	rand.Seed(4)

	var (
		require   = require.New(t)
		numKeys   = 5
		numTxs    = 100_000
		completed = make([]int, 0, numTxs)
		answers   = make([][]int, 5)
		e         = New(numTxs, 4, maxDependencies, nil)

		numBlocking  = 10000
		blocking     = set.Set[int]{}
		conflictKeys = make([]string, 0, numKeys)

		l    sync.Mutex
		slow = make(chan struct{})
	)

	// make [answers] matrix
	answers[0] = generateNumbers(0)
	answers[1] = generateNumbers(20000)
	answers[2] = generateNumbers(40000)
	answers[3] = generateNumbers(60000)
	answers[4] = generateNumbers(80000)

	// randomly wait on certain txs
	for blocking.Len() < numBlocking {
		blocking.Add(rand.Intn(numTxs)) //nolint:gosec
	}

	// make the conflict keys
	for k := 0; k < numKeys; k++ {
		conflictKeys = append(conflictKeys, ids.GenerateTestID().String())
	}

	// Generate txs
	for i := 0; i < numTxs; i++ {
		s := make(state.Keys, (numKeys + 1))
		for j := 0; j < numKeys; j++ {
			mode := (i / 10000) % 2
			switch mode {
			case 0:
				s.Add(conflictKeys[j], state.Write)
			case 1:
				s.Add(conflictKeys[j], state.Read)
			}
		}

		// pass into executor
		ti := i
		e.Run(s, func() error {
			if blocking.Contains(ti) {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	for i := 0; i < numBlocking; i++ {
		slow <- struct{}{}
	}
	close(slow)
	require.NoError(e.Wait())
	require.Len(completed, numTxs)
	require.Equal(answers[0], completed[0:9999])
	require.Equal(answers[1], completed[20000:29999])
	require.Equal(answers[2], completed[40000:49999])
	require.Equal(answers[3], completed[60000:69999])
	require.Equal(answers[4], completed[80000:89999])
}

// Random mix of unique and conflicting keys. We just
// need to see if we don't halt in this test
func TestLargeRandomReadsAndWrites(t *testing.T) {
	rand.Seed(5)

	var (
		require   = require.New(t)
		numKeys   = 10
		numTxs    = 100000
		completed = make([]int, 0, numTxs)
		e         = New(numTxs, 4, maxDependencies, nil)

		conflictSize = 100
		conflictKeys = make([]string, 0, conflictSize)

		numBlocking = 10000
		blocking    = set.Set[int]{}

		l    sync.Mutex
		slow = make(chan struct{})
	)

	// randomly wait on certain txs
	for blocking.Len() < numBlocking {
		blocking.Add(rand.Intn(numTxs)) //nolint:gosec
	}

	// make the conflict keys
	for k := 0; k < conflictSize; k++ {
		conflictKeys = append(conflictKeys, ids.GenerateTestID().String())
	}

	// Generate txs
	for i := 0; i < numTxs; i++ {
		// mix of unique and conflict keys in each tx
		s := make(state.Keys, (numKeys + 1))

		// random size of conflict keys to add
		setSize := max(1, rand.Intn(6))                   //nolint:gosec
		randomConflictingKeys := set.NewSet[int](setSize) // indices of [conflictKeys]
		for randomConflictingKeys.Len() < setSize {
			randomConflictingKeys.Add(rand.Intn(conflictSize)) //nolint:gosec
		}

		// add the random keys to tx
		for k := range randomConflictingKeys {
			// randomly pick if conflict key is Read/Write
			switch rand.Intn(2) { //nolint:gosec
			case 0:
				s.Add(conflictKeys[k], state.Read)
			case 1:
				s.Add(conflictKeys[k], state.Write)
			}
		}

		// fill in rest with unique keys
		remaining := numKeys - setSize
		for j := 0; j < remaining; j++ {
			// randomly pick the permission for unique keys
			switch rand.Intn(2) { //nolint:gosec
			case 0:
				s.Add(ids.GenerateTestID().String(), state.Read)
			case 1:
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
		}

		// pass into executor
		ti := i
		e.Run(s, func() error {
			if blocking.Contains(ti) {
				<-slow
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	for i := 0; i < numBlocking; i++ {
		slow <- struct{}{}
	}
	close(slow)
	require.NoError(e.Wait())
	require.Len(completed, numTxs)
}
