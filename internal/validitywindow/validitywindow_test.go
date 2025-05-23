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

func newPopulatedValidityWindow(ctx context.Context, r *require.Assertions, blocks []executionBlock, head executionBlock, validityWindowDuration int64) *TimeValidityWindow[container] {
	chainIndex := &testChainIndex{}
	for _, blk := range blocks {
		chainIndex.set(blk.GetID(), blk)
	}

	validityWindow, err := NewTimeValidityWindow(ctx, &logging.NoLog{}, trace.Noop, chainIndex, head, func(int64) int64 {
		return validityWindowDuration
	})
	r.NoError(err)

	return validityWindow
}

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
			ctx := context.Background()

			validityWindow := newPopulatedValidityWindow(ctx, r, test.blocks, test.blocks[test.accepted], test.validityWindow)
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
			ctx := context.Background()

			validityWindow := newPopulatedValidityWindow(ctx, r, test.blocks, test.blocks[test.accepted], test.validityWindow)
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

func TestVerifyTimestamp(t *testing.T) {
	tests := []struct {
		name               string
		containerTimestamp int64
		executionTimestamp int64
		divisor            int64
		validityWindow     int64
		expectedErr        error
	}{
		{
			name:               "container ts = execution ts",
			containerTimestamp: 10,
			executionTimestamp: 10,
			divisor:            1,
			validityWindow:     10,
		},
		{
			name:               "container expired",
			containerTimestamp: 9,
			executionTimestamp: 10,
			divisor:            1,
			validityWindow:     10,
			expectedErr:        ErrTimestampExpired,
		},
		{
			name:               "container ts inside validity window",
			containerTimestamp: 11,
			executionTimestamp: 10,
			divisor:            1,
			validityWindow:     10,
		},
		{
			name:               "container ts past validity window",
			containerTimestamp: 21,
			executionTimestamp: 10,
			divisor:            1,
			validityWindow:     10,
			expectedErr:        ErrFutureTimestamp,
		},
		{
			name:               "container ts is not multiple of divisor",
			containerTimestamp: 11,
			executionTimestamp: 10,
			divisor:            2,
			validityWindow:     10,
			expectedErr:        ErrMisalignedTime,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := require.New(t)
			err := VerifyTimestamp(test.containerTimestamp, test.executionTimestamp, test.divisor, test.validityWindow)
			r.ErrorIs(err, test.expectedErr)
		})
	}
}

// TestValidityWindowBoundaryLifespan tests that a container included at the validity window boundary transitions
// seamlessly from failing verification due to a duplicate within the validity window to failing because it expired.
func TestValidityWindowBoundaryLifespan(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	// Create accepted genesis block
	chainIndex := &testChainIndex{}
	genesisBlk := newExecutionBlock(0, 0, []int64{1})
	chainIndex.set(genesisBlk.GetID(), genesisBlk)

	validityWindowDuration := int64(10)
	validityWindow, err := NewTimeValidityWindow[container](ctx, &logging.NoLog{}, trace.Noop, chainIndex, genesisBlk, func(int64) int64 {
		return validityWindowDuration
	})
	r.NoError(err)
	validityWindow.Accept(genesisBlk)

	blk1 := newExecutionBlock(1, 0, []int64{validityWindowDuration})
	blk2 := newExecutionBlock(2, validityWindowDuration, []int64{validityWindowDuration})

	// Verify a timestamp at the validity window boundary
	r.NoError(VerifyTimestamp(validityWindowDuration, 0, 1, validityWindowDuration))

	// Including the first block should pass
	r.NoError(validityWindow.VerifyExpiryReplayProtection(context.Background(), blk1))
	chainIndex.set(blk1.GetID(), blk1)

	// Verify a timestamp at the validity window boundary fails for both a processing
	// and accepted parent.
	// Processing:
	r.ErrorIs(validityWindow.VerifyExpiryReplayProtection(context.Background(), blk2), ErrDuplicateContainer)

	// Accepted:
	validityWindow.Accept(blk1)
	r.ErrorIs(validityWindow.VerifyExpiryReplayProtection(context.Background(), blk2), ErrDuplicateContainer)

	// Verify that after passing the validity window, the timestamp is no longer valid
	r.ErrorIs(VerifyTimestamp(validityWindowDuration, validityWindowDuration+1, 1, validityWindowDuration), ErrTimestampExpired)
}

func TestAcceptHistorical(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	// Create and accept the genesis block to set an initial lastAcceptedBlockHeight
	chainIndex := &testChainIndex{}
	genesisBlk := newExecutionBlock(0, 0, []int64{})
	chainIndex.set(genesisBlk.GetID(), genesisBlk)

	validityWindowDuration := int64(10)
	validityWindow, err := NewTimeValidityWindow(ctx, &logging.NoLog{}, trace.Noop, chainIndex, genesisBlk, func(int64) int64 {
		return validityWindowDuration
	})
	r.NoError(err)
	validityWindow.Accept(genesisBlk)
	r.Equal(uint64(0), validityWindow.lastAcceptedBlockHeight)

	// Create a block at height 1 and accept it
	blk1 := newExecutionBlock(1, 5, []int64{1, 2})
	chainIndex.set(blk1.GetID(), blk1)
	validityWindow.Accept(blk1)
	r.Equal(uint64(1), validityWindow.lastAcceptedBlockHeight)

	// Create a historical block at height 5 (higher than current accepted)
	historicalBlk := newExecutionBlock(5, 20, []int64{5, 6})
	chainIndex.set(historicalBlk.GetID(), historicalBlk)
	validityWindow.AcceptHistorical(historicalBlk)
	// Verify that lastAcceptedBlockHeight has not changed
	r.Equal(uint64(1), validityWindow.lastAcceptedBlockHeight)
}

type testChainIndex struct {
	beforeSaveFunc func(blocks map[ids.ID]ExecutionBlock[container]) error
	blocks         map[ids.ID]ExecutionBlock[container]
}

func (t *testChainIndex) GetExecutionBlock(_ context.Context, blkID ids.ID) (ExecutionBlock[container], error) {
	if blk, ok := t.blocks[blkID]; ok {
		return blk, nil
	}
	return nil, database.ErrNotFound
}

func (t *testChainIndex) SaveHistorical(blk ExecutionBlock[container]) error {
	if t.blocks == nil {
		t.blocks = make(map[ids.ID]ExecutionBlock[container])
	}

	if t.beforeSaveFunc != nil {
		return t.beforeSaveFunc(t.blocks)
	}
	t.blocks[blk.GetID()] = blk
	return nil
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
	Bytes  []byte
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

func (e executionBlock) GetBytes() []byte {
	return e.Bytes
}

func newExecutionBlock(height uint64, timestamp int64, containers []int64) executionBlock {
	id := uint64ToID(height)

	e := executionBlock{
		Prnt:   uint64ToID(height - 1), // Allow underflow for genesis
		Tmstmp: timestamp,
		Hght:   height,
		ID:     id,
		Bytes:  binary.BigEndian.AppendUint64(nil, height),
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
