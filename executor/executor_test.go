// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"
	"testing"
	"time"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state"
)

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
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 100)
		e         = New(100, 4, nil)
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Read|state.Write)
		}
		ti := i
		e.Run(s, func() error {
			if ti == 0 {
				time.Sleep(3 * time.Second)
			}
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait()) // existing task is running
	require.Len(completed, 100)
	require.Equal(0, completed[99])
}

func TestExecutorSimpleConflict(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
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
				time.Sleep(3 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
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
				time.Sleep(3 * time.Second)
			}
			if ti == 15 {
				time.Sleep(5 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
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
	require.Less(len(completed), 500)
	require.ErrorIs(e.Wait(), terr) // no task running
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
	require.Less(len(completed), 500)
	require.ErrorIs(e.Wait(), ErrStopped) // no task running
}

// W->W->W->...
/*func TestManyWrites(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		answer      = make([]int, 0, 100)
		e           = New(100, 4, nil)
	)
	for i := 0; i < 100; i++ {
		answer = append(answer, i)
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Write)
		}
		s.Add(conflictKey, state.Write)
		ti := i
		e.Run(s, func() error {
			if ti == 0 {
				time.Sleep(3 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	require.Equal(answer, completed)
}

// R->R->R->...
func TestManyReads(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Write)
		}
		s.Add(conflictKey, state.Read)
		ti := i
		e.Run(s, func() error {
			if ti%2 == 0 {
				time.Sleep(1 * time.Second)
			}
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	// 0..99 are ran in parallel, so non-deterministic
	require.Len(completed, 100)
}

// W->R->R->...
func TestWriteThenRead(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Write)
		}
		if i == 0 {
			s.Add(conflictKey, state.Write)
		} else {
			s.Add(conflictKey, state.Read)
		}
		ti := i
		e.Run(s, func() error {
			if ti == 0 {
				time.Sleep(1 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	fmt.Printf("completed %v\n", completed)
	require.Equal(0, completed[0]) // Write first to execute
	// 1..99 are ran in parallel, so non-deterministic
	require.Len(completed, 100)
}

// R->R->W...
func TestReadThenWrite(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
	)
	for i := 0; i < 100; i++ {
		s := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String(), state.Write)
		}
		if i == 10 {
			s.Add(conflictKey, state.Write)
		} else {
			s.Add(conflictKey, state.Read)
		}
		ti := i
		e.Run(s, func() error {
			if ti == 10 {
				time.Sleep(1 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	fmt.Printf("completed %v\n", completed)
	// 0..9 are ran in parallel, so non-deterministic
	require.Equal(10, completed[10]) // First write to execute
	// 11..99 are ran in parallel, so non-deterministic
	require.Len(completed, 100)
}

// W->R->R->...W->R->R->...
func TestWriteThenReadRepeated(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
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
				time.Sleep(1 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	fmt.Printf("completed %v\n", completed)
	require.Equal(0, completed[0]) // First write to execute
	// 1..48 are ran in parallel, so non-deterministic
	require.Equal(49, completed[49]) // Second write to execute
	// 50..99 are ran in parallel, so non-deterministic
	require.Len(completed, 100)
}

// R->R->W->R->W->R->R...
func TestReadThenWriteRepeated(t *testing.T) {
	var (
		require     = require.New(t)
		conflictKey = ids.GenerateTestID().String()
		l           sync.Mutex
		completed   = make([]int, 0, 100)
		e           = New(100, 4, nil)
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
				time.Sleep(1 * time.Second)
			}

			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
			return nil
		})
	}
	require.NoError(e.Wait())
	fmt.Printf("completed %v\n", completed)
	// 0..9 are ran in parallel, so non-deterministic
	require.Equal(10, completed[10])
	require.Equal(11, completed[11])
	require.Equal(12, completed[12])
	// 13..99 are ran in parallel, so non-deterministic
	require.Len(completed, 100)
}
*/
