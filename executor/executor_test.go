// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		if i%10 == 0 {
			s.Add(conflictKey)
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		if i%10 == 0 {
			s.Add(conflictKey)
		}
		if i == 15 || i == 20 {
			s.Add(conflictKey2)
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
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
	require.True(len(completed) < 500)
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
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
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
	require.True(len(completed) < 500)
	require.ErrorIs(e.Wait(), ErrStopped) // no task running
}
