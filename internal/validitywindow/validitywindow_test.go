// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
)

func TestValidityWindowVerifyExpiryReplayProtection(t *testing.T) {
	tests := []struct {
		name           string
		blocks         []executionBlock
		accepted       int // index into Blocks of the last accepted block.
		verifyBlock    executionBlock
		validityWindow int64
		expectedError  error
	}{
		{
			name: "no duplicate",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       1,
			verifyBlock:    newExecutionBlock(2, 3, []int64{3}),
			validityWindow: 5,
		},
		{
			name: "expected duplicate within block",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{0}),
				newExecutionBlock(1, 1, []int64{1}),
			},
			accepted:       1,
			verifyBlock:    newExecutionBlock(2, 3, []int64{2, 2}),
			validityWindow: 5,
			expectedError:  ErrDuplicateContainer,
		},
		{
			name: "expected duplicate within boundary (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       1,
			verifyBlock:    newExecutionBlock(2, 2, []int64{2}),
			validityWindow: 5,
			expectedError:  ErrDuplicateContainer,
		},
		{
			name: "expected duplicate within boundary (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       0,
			verifyBlock:    newExecutionBlock(2, 2, []int64{2}),
			validityWindow: 5,
			expectedError:  ErrDuplicateContainer,
		},
		{
			name: "expected duplicate at boundary (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       1,
			verifyBlock:    newExecutionBlock(2, 2, []int64{2}),
			validityWindow: 1,
			expectedError:  ErrDuplicateContainer,
		},
		{
			name: "expected duplicate at boundary (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       0,
			verifyBlock:    newExecutionBlock(2, 2, []int64{2}),
			validityWindow: 1,
			expectedError:  ErrDuplicateContainer,
		},
		{
			name: "duplicate outside window (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       1,
			verifyBlock:    newExecutionBlock(2, 3, []int64{2}),
			validityWindow: 1,
			expectedError:  nil,
		},
		{
			name: "duplicate outside window (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:       0,
			verifyBlock:    newExecutionBlock(2, 3, []int64{2}),
			validityWindow: 1,
			expectedError:  nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			chainIndex := &testChainIndex{}
			validityWindow := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex, func(int64) int64 {
				return test.validityWindow
			})
			for i, blk := range test.blocks {
				if i <= test.accepted {
					validityWindow.Accept(blk)
				}
				chainIndex.set(blk.GetID(), blk)
			}
			r.ErrorIs(validityWindow.VerifyExpiryReplayProtection(context.Background(), test.verifyBlock), test.expectedError)
		})
	}
}

func TestValidityWindowIsRepeat(t *testing.T) {
	tests := []struct {
		name   string
		blocks []executionBlock
		// if non-nil, use this block as the parent block instead of
		// the last item in blocks.
		// Used to test missing chain index.
		overrideParentBlock func() executionBlock
		accepted            int // index into Blocks of the last accepted block.
		containers          []container
		currentTimestamp    int64
		validityWindow      int64
		expectedError       error
		expectedBits        set.Bits
	}{
		{
			name: "no containers",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted:         1,
			containers:       []container{},
			currentTimestamp: 2,
			validityWindow:   1,
			expectedError:    nil,
			expectedBits:     set.NewBits(),
		},
		{
			name: "no repeats (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted: 1,
			containers: []container{
				newContainer(5),
			},
			currentTimestamp: 2,
			validityWindow:   5,
			expectedError:    nil,
			expectedBits:     set.NewBits(),
		},
		{
			name: "no repeats (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted: 0,
			containers: []container{
				newContainer(5),
			},
			currentTimestamp: 2,
			validityWindow:   5,
			expectedError:    nil,
			expectedBits:     set.NewBits(),
		},
		{
			name: "repeats outside validity window (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{1}),
				newExecutionBlock(1, 1, []int64{2}),
			},
			accepted: 1,
			containers: []container{
				newContainer(1),
			},
			currentTimestamp: 3,
			validityWindow:   1,
			expectedError:    nil,
			expectedBits:     set.NewBits(),
		},
		{
			name: "repeats outside validity window (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{1}),
				newExecutionBlock(1, 1, []int64{2}),
			},
			accepted: 0,
			containers: []container{
				newContainer(1),
			},
			currentTimestamp: 2,
			validityWindow:   1,
			expectedError:    nil,
			expectedBits:     set.NewBits(),
		},
		{
			name: "repeats within validity window parent (accepted)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted: 1,
			containers: []container{
				newContainer(1),
			},
			currentTimestamp: 2,
			validityWindow:   2,
			expectedError:    nil,
			expectedBits:     set.NewBits(0),
		},
		{
			name: "repeats within validity window parent (processing)",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1, 2}),
			},
			accepted: 0,
			containers: []container{
				newContainer(1),
			},
			currentTimestamp: 2,
			validityWindow:   2,
			expectedError:    nil,
			expectedBits:     set.NewBits(0),
		},
		{
			name: "missing block in ancestery",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, []int64{}),
				newExecutionBlock(1, 1, []int64{1}),
				newExecutionBlock(2, 2, []int64{2}),
			},
			accepted: 1,
			overrideParentBlock: func() executionBlock {
				return newExecutionBlock(5, 5, []int64{})
			},
			containers: []container{
				newContainer(3),
			},
			currentTimestamp: 5,
			validityWindow:   4,
			expectedError:    database.ErrNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			chainIndex := &testChainIndex{}
			validityWindow := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex, func(timestamp int64) int64 {
				return test.validityWindow
			})
			for i, blk := range test.blocks {
				if i <= test.accepted {
					validityWindow.Accept(blk)
				}
				chainIndex.set(blk.GetID(), blk)
			}
			parent := test.blocks[len(test.blocks)-1]
			if test.overrideParentBlock != nil {
				parent = test.overrideParentBlock()
			}
			bits, err := validityWindow.IsRepeat(context.Background(), parent, test.currentTimestamp, test.containers)
			r.ErrorIs(err, test.expectedError)
			if err != nil {
				return
			}

			r.Equal(test.expectedBits.Bytes(), bits.Bytes())
		})
	}
}

type testChainIndex struct {
	blocks map[ids.ID]ExecutionBlock[container]
}

func (t testChainIndex) GetExecutionBlock(_ context.Context, blkID ids.ID) (ExecutionBlock[container], error) {
	if blk, ok := t.blocks[blkID]; ok {
		return blk, nil
	}
	return nil, database.ErrNotFound
}

func (t *testChainIndex) set(blkID ids.ID, blk ExecutionBlock[container]) {
	if t.blocks == nil {
		t.blocks = make(map[ids.ID]ExecutionBlock[container])
	}
	t.blocks[blkID] = blk
}

type container struct {
	ID     ids.ID
	Expiry int64
}

func (c container) GetID() ids.ID {
	return c.ID
}

func (c container) GetExpiry() int64 {
	return c.Expiry
}

func newContainer(expiry int64) container {
	return container{
		Expiry: expiry,
		ID:     uint64ToID(uint64(expiry)),
	}
}

type executionBlock struct {
	Prnt   ids.ID
	Tmstmp int64
	Hght   uint64
	Ctrs   []container
	ID     ids.ID
}

func (e executionBlock) GetID() ids.ID {
	return e.ID
}

func (e executionBlock) GetParent() ids.ID {
	return e.Prnt
}

func (e executionBlock) GetTimestamp() int64 {
	return e.Tmstmp
}

func (e executionBlock) GetHeight() uint64 {
	return e.Hght
}

func (e executionBlock) GetContainers() []container {
	return e.Ctrs
}

func (e executionBlock) Contains(id ids.ID) bool {
	for _, c := range e.Ctrs {
		if c.GetID() == id {
			return true
		}
	}
	return false
}

func newExecutionBlock(height uint64, timestamp int64, containers []int64) executionBlock {
	e := executionBlock{
		Prnt:   uint64ToID(height - 1), // Allow underflow for genesis
		Tmstmp: timestamp,
		Hght:   height,
		ID:     uint64ToID(height),
	}
	for _, c := range containers {
		e.Ctrs = append(e.Ctrs, newContainer(c))
	}
	return e
}

func uint64ToID(n uint64) ids.ID {
	var id ids.ID
	binary.BigEndian.PutUint64(id[:], n)
	return id
}
