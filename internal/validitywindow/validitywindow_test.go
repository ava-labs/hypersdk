// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
)

func TestValidityWindowVerifyExpiryReplayProtection(t *testing.T) {
	tests := []struct {
		Name          string
		Accepted      []validitywindowtest.ExecutionBlock
		VerifyBlock   validitywindowtest.ExecutionBlock
		OldestAllowed int64
		ExpectedError error
	}{
		{
			Name: "no duplicate",
			Accepted: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			VerifyBlock:   validitywindowtest.NewExecutionBlock(2, 3, 3, 3),
			OldestAllowed: 1,
		},
		{
			Name: "expected duplicate",
			Accepted: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			VerifyBlock:   validitywindowtest.NewExecutionBlock(2, 3, 3, 2),
			OldestAllowed: 1,
			ExpectedError: ErrDuplicateContainer,
		},
		{
			Name: "duplicate outside window",
			Accepted: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			VerifyBlock:   validitywindowtest.NewExecutionBlock(2, 3, 3, 2),
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
		Blocks         []validitywindowtest.ExecutionBlock
		Accepted       uint64 // index into Blocks of the last accepted block.
		ParentBlock    validitywindowtest.ExecutionBlock
		Containers     []validitywindowtest.Container
		OldestAllowewd int64
		ExpectedError  error
		ExpectedBits   set.Bits
	}{
		{
			Name: "no containers",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			Accepted:       1,
			ParentBlock:    validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			Containers:     []validitywindowtest.Container{},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "no repeats",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			Accepted:    1,
			ParentBlock: validitywindowtest.NewExecutionBlock(2, 2, 2, 3),
			Containers: []validitywindowtest.Container{
				validitywindowtest.NewContainer(5),
			},
			OldestAllowewd: 0,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "repeats outside validity window",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			Accepted:    1,
			ParentBlock: validitywindowtest.NewExecutionBlock(2, 2, 2, 3),
			Containers: []validitywindowtest.Container{
				validitywindowtest.NewContainer(1),
			},
			OldestAllowewd: 2,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(),
		},
		{
			Name: "repeats within latest accepted validity window block",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1, 2),
			},
			Accepted:    1,
			ParentBlock: validitywindowtest.NewExecutionBlock(2, 2, 2, 3),
			Containers: []validitywindowtest.Container{
				validitywindowtest.NewContainer(1),
			},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(0),
		},
		{
			Name: "repeats after latest accepted block",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1),
				validitywindowtest.NewExecutionBlock(2, 2, 2, 2),
				validitywindowtest.NewExecutionBlock(3, 3, 3, 3),
				validitywindowtest.NewExecutionBlock(4, 4, 4, 4),
			},
			Accepted:    1,
			ParentBlock: validitywindowtest.NewExecutionBlock(5, 5, 5),
			Containers: []validitywindowtest.Container{
				validitywindowtest.NewContainer(3),
			},
			OldestAllowewd: 1,
			ExpectedError:  nil,
			ExpectedBits:   set.NewBits(0),
		},
		{
			Name: "missing block in ancestery",
			Blocks: []validitywindowtest.ExecutionBlock{
				validitywindowtest.NewExecutionBlock(0, 0, 0),
				validitywindowtest.NewExecutionBlock(1, 1, 1, 1),
				validitywindowtest.NewExecutionBlock(2, 2, 2, 2),
			},
			Accepted:    1,
			ParentBlock: validitywindowtest.NewExecutionBlock(5, 5, 5),
			Containers: []validitywindowtest.Container{
				validitywindowtest.NewContainer(3),
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

// testing structures.
type testChainIndex struct {
	blocks map[ids.ID]ExecutionBlock[validitywindowtest.Container]
}

func (t testChainIndex) GetExecutionBlock(_ context.Context, blkID ids.ID) (ExecutionBlock[validitywindowtest.Container], error) {
	if blk, ok := t.blocks[blkID]; ok {
		return blk, nil
	}
	return nil, database.ErrNotFound
}

func (t *testChainIndex) set(blkID ids.ID, blk ExecutionBlock[validitywindowtest.Container]) {
	if t.blocks == nil {
		t.blocks = make(map[ids.ID]ExecutionBlock[validitywindowtest.Container])
	}
	t.blocks[blkID] = blk
}
