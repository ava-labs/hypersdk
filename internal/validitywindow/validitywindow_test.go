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
		Name          string
		Accepted      []executionBlock
		VerifyBlock   executionBlock
		OldestAllowed int64
		ExpectedError error
	}{
		{
			Name: "no duplicate",
			Accepted: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			VerifyBlock:   newExecutionBlock(2, 3, 3, []int64{3}),
			OldestAllowed: 1,
		},
		{
			Name: "expected duplicate",
			Accepted: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			VerifyBlock:   newExecutionBlock(2, 3, 3, []int64{2}),
			OldestAllowed: 1,
			ExpectedError: ErrDuplicateContainer,
		},
		{
			Name: "duplicate outside window",
			Accepted: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			VerifyBlock:   newExecutionBlock(2, 3, 3, []int64{2}),
			OldestAllowed: 2,
			ExpectedError: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			r := require.New(t)
			r.NoError(nil)

			chainIndex := &testChainIndex{}
			tvw := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex)
			r.NotNil(tvw)
			for _, acceptedBlk := range test.Accepted {
				tvw.Accept(acceptedBlk)
				chainIndex.set(acceptedBlk.GetID(), acceptedBlk)
			}
			r.ErrorIs(tvw.VerifyExpiryReplayProtection(context.Background(), test.VerifyBlock, test.OldestAllowed), test.ExpectedError)
		})
	}
}

func TestValidityWindowIsRepeat(t *testing.T) {
	tests := []struct {
		Name           string
		Blocks         []executionBlock
		Accepted       uint64 // index into Blocks of the last accepted block.
		ParentBlock    executionBlock
		Containers     []container
		OldestAllowewd int64
		ExpectedError  error
		ExpectedBits   set.Bits
	}{
		{
			Name: "no containers",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			Accepted:       1,
			ParentBlock:    newExecutionBlock(1, 1, 1, []int64{1, 2}),
			Containers:     []container{},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "no repeats",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			Accepted:    1,
			ParentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			Containers: []container{
				newContainer(5),
			},
			OldestAllowewd: 0,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "repeats outside validity window",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			Accepted:    1,
			ParentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			Containers: []container{
				newContainer(1),
			},
			OldestAllowewd: 2,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "repeats within latest accepted validity window block",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1, 2}),
			},
			Accepted:    1,
			ParentBlock: newExecutionBlock(2, 2, 2, []int64{3}),
			Containers: []container{
				newContainer(1),
			},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(0),
		},
		{
			Name: "repeats after latest accepted block",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1}),
				newExecutionBlock(2, 2, 2, []int64{2}),
				newExecutionBlock(3, 3, 3, []int64{3}),
				newExecutionBlock(4, 4, 4, []int64{4}),
			},
			Accepted:    1,
			ParentBlock: newExecutionBlock(5, 5, 5, []int64{}),
			Containers: []container{
				newContainer(3),
			},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(0),
		},
		{
			Name: "missing block in ancestery",
			Blocks: []executionBlock{
				newExecutionBlock(0, 0, 0, []int64{}),
				newExecutionBlock(1, 1, 1, []int64{1}),
				newExecutionBlock(2, 2, 2, []int64{2}),
			},
			Accepted:    1,
			ParentBlock: newExecutionBlock(5, 5, 5, []int64{}),
			Containers: []container{
				newContainer(3),
			},
			OldestAllowewd: 1,
			ExpectedError:  database.ErrNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			r := require.New(t)
			r.NoError(nil)

			chainIndex := &testChainIndex{}
			tvw := NewTimeValidityWindow(&logging.NoLog{}, trace.Noop, chainIndex)
			r.NotNil(tvw)
			for i, blk := range test.Blocks {
				if i <= int(test.Accepted) {
					tvw.Accept(blk)
				}
				chainIndex.set(blk.GetID(), blk)
			}
			bits, err := tvw.IsRepeat(context.Background(), test.ParentBlock, test.Containers, test.OldestAllowewd)
			r.ErrorIs(err, test.ExpectedError)
			if err == nil {
				r.Equal(test.ExpectedBits.Bytes(), bits.Bytes())
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

func (e executionBlock) Parent() ids.ID {
	return e.Prnt
}

func (e executionBlock) Timestamp() int64 {
	return e.Tmstmp
}

func (e executionBlock) Height() uint64 {
	return e.Hght
}

func (e executionBlock) Containers() []container {
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
