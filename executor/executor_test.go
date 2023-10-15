package executor

import (
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
		e         = New(100, 4)
		canWait   = make(chan struct{})
	)
	for i := 0; i < 100; i++ {
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		ti := i
		e.Run(s, func() {
			l.Lock()
			completed = append(completed, ti)
			if len(completed) == 100 {
				close(canWait)
			}
			l.Unlock()
		})
	}
	<-canWait
	e.Wait() // no task running
	require.Len(completed, 100)
}

func TestExecutorNoConflictsSlow(t *testing.T) {
	var (
		require   = require.New(t)
		l         sync.Mutex
		completed = make([]int, 0, 100)
		e         = New(100, 4)
	)
	for i := 0; i < 100; i++ {
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		ti := i
		e.Run(s, func() {
			if ti == 0 {
				time.Sleep(3 * time.Second)
			}
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
		})
	}
	e.Wait() // existing task is running
	require.Len(completed, 100)
	require.Equal(0, completed[99])
}

func TestExecutorConflicts(t *testing.T) {
	var (
		require   = require.New(t)
		expected  = make([]int, 0, 100)
		l         sync.Mutex
		completed = make([]int, 0, 100)
		e         = New(100, 4)
	)
	for i := 0; i < 100; i++ {
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		ti := i
		e.Run(s, func() {
			l.Lock()
			completed = append(completed, ti)
			l.Unlock()
		})
	}
	e.Wait()
	require.Equal(expected, completed)
}
