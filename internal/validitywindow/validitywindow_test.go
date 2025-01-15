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
		name          string
		blocks        []executionBlock
		accepted      int // index into Blocks of the last accepted block.
		verifyBlock   executionBlock
		oldestAllowed int64
		expectedError error
	}{
		{
			name: "no duplicate",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:      1,
			verifyBlock:   newExecutionBlock(2, 3, 3, []int64{3}),
			oldestAllowed: 1,
		},
		{
			name: "expected duplicate",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:      1,
			verifyBlock:   newExecutionBlock(2, 3, 3, []int64{2}),
			oldestAllowed: 1,
			expectedError: ErrDuplicateContainer,
		},
		{
			name: "duplicate outside window",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:      1,
			verifyBlock:   newExecutionBlock(2, 3, 3, []int64{2}),
			oldestAllowed: 2,
			expectedError: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			chainIndex := &testChainIndex{}
			tvw := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex)
			r.NotNil(tvw)
			for i, blk := range test.blocks {
				if i <= test.accepted {
					tvw.Accept(blk)
				}
				chainIndex.set(blk.GetID(), blk)
			}
			r.ErrorIs(tvw.VerifyExpiryReplayProtection(context.Background(), test.verifyBlock, test.oldestAllowed), test.expectedError)
		})
	}
}

func TestValidityWindowIsRepeat(t *testing.T) {
	tests := []struct {
		name           string
		blocks         []executionBlock
		accepted       int // index into Blocks of the last accepted block.
		parentBlock    executionBlock
		containers     []container
		oldestAllowewd int64
		expectedError  error
		expectedBits   set.Bits
	}{
		{
			name: "no containers",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:       1,
			parentBlock:    newExecutionBlock(1, 1, 1, []int64{1, 2}),
			containers:     []container{},
			oldestAllowewd: 1,
			expectedError:  nil,
			expectedBits:   set.NewBits(),
		},
		{
			name: "no repeats",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:    1,
			parentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			containers: []container{
				newContainer(5),
			},
			oldestAllowewd: 0,
			expectedError:  nil,
			expectedBits:   set.NewBits(),
		},
		{
			name: "repeats outside validity window",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:    1,
			parentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			containers: []container{
				newContainer(1),
			},
			oldestAllowewd: 2,
			expectedError:  nil,
			expectedBits:   set.NewBits(),
		},
		{
			name: "repeats within latest accepted validity window block",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			accepted:    1,
			parentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			containers: []container{
				newContainer(1),
			},
			oldestAllowewd: 1,
			expectedError:  nil,
			expectedBits:   set.NewBits(0),
		},
		{
			name: "repeats after latest accepted block",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1}),
				newExecutionBlock(2, 2, 2, []int64{2}),
				newExecutionBlock(3, 3, 3, []int64{3}),
				newExecutionBlock(4, 4, 4, []int64{4}),
			},
			accepted:    1,
			parentBlock: newExecutionBlock(5, 5, 5, []int64{}),
			containers: []container{
				newContainer(3),
			},
			oldestAllowewd: 1,
			expectedError:  nil,
			expectedBits:   set.NewBits(0),
		},
		{
			name: "missing block in ancestery",
			blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1}),
				newExecutionBlock(2, 2, 2, []int64{2}),
			},
			accepted:    1,
			parentBlock: newExecutionBlock(5, 5, 5, []int64{}),
			containers: []container{
				newContainer(3),
			},
			oldestAllowewd: 1,
			expectedError:  database.ErrNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)

			chainIndex := &testChainIndex{}
			tvw := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex)
			r.NotNil(tvw)
			for i, blk := range test.blocks {
				if i <= test.accepted {
					tvw.Accept(blk)
				}
				chainIndex.set(blk.GetID(), blk)
			}
			bits, err := tvw.IsRepeat(context.Background(), test.parentBlock, test.containers, test.oldestAllowewd)
			r.ErrorIs(err, test.expectedError)
			if err == nil {
				r.Equal(test.expectedBits.Bytes(), bits.Bytes())
			}
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
		ID:     int64ToID(expiry),
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

func newExecutionBlock(parent int64, timestamp int64, height uint64, containers []int64) executionBlock {
	e := executionBlock{
		Prnt:   int64ToID(parent),
		Tmstmp: timestamp,
		Hght:   height,
		ID:     int64ToID(parent + 1),
	}
	for _, c := range containers {
		e.Ctrs = append(e.Ctrs, newContainer(c))
	}
	return e
}

func int64ToID(n int64) ids.ID {
	var id ids.ID
	binary.BigEndian.PutUint64(id[0:8], uint64(n))
	return id
}
