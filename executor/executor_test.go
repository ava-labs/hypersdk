package executor

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
)

func TestExecutorNoConflicts(t *testing.T) {
	require := require.New(t)
	expected := make([]int, 0, 100)
	var l sync.Mutex
	completed := make([]int, 0, 100)
	e := New(100, 4)
	for i := 0; i < 100; i++ {
		expected = append(expected, i)
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

func TestExecutorConflicts(t *testing.T) {
	e := New(100, 4)
	for i := 0; i < 100; i++ {
		s := set.NewSet[string](i + 1)
		for k := 0; k < i+1; k++ {
			s.Add(ids.GenerateTestID().String())
		}
		ti := i
		e.Run(s, func() {
			fmt.Println("executing:", ti)
		})
	}
	e.Wait()
}
