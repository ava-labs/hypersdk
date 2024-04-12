// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state"
)

// Run several times to catch non-determinism
const numIterations = 10

func TestExecutorNoConflicts(t *testing.T) {
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 100)
		e         = New(100, 4, nil)
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
			e         = New(100, 4, nil)
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
		e           = New(100, 4, nil)
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
		e            = New(100, 4, nil)
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
		e         = New(500, 4, nil)
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
		e         = New(500, 4, nil)
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
			e           = New(100, 4, nil)
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
			e           = New(100, 4, nil)
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
			e           = New(100, 4, nil)
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
			e           = New(100, 4, nil)
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
			e           = New(100, 4, nil)
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
			e           = New(100, 4, nil)
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
			e            = New(100, 4, nil)
			slow         = make(chan struct{})
		)
		for i := 0; i < 100; i++ {
			s := make(state.Keys, (i + 1))
			for k := 0; k < i+1; k++ {
				s.Add(ids.GenerateTestID().String(), state.Write)
			}
			if i == 0 || i == 1 {
				s.Add(conflictKey1, state.Write)
				s.Add(conflictKey2, state.Write)
			} else {
				s.Add(conflictKey1, state.Read)
				s.Add(conflictKey2, state.Read)
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
		require.Equal(0, completed[0])
		require.Equal(1, completed[1])
		// 2..99 are ran in parallel, so non-deterministic
		require.Len(completed, 100)
	}
}
