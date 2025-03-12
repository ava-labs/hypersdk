// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

var (
	_ Rules       = (*rules)(nil)
	_ RuleFactory = (*ruleFactory)(nil)
)

func TestPendingChunkStore(t *testing.T) {
	r := require.New(t)

	db := memdb.New()
	ruleFactory := newRuleFactory(100, 60000)
	store, err := newPendingChunkStore(db, ruleFactory)
	r.NoError(err)
	r.NotNil(store)

	now := time.Now()
	chunk1 := NewChunk(ids.GenerateTestNodeID(), now.Add(5*time.Minute).UnixMilli(), []byte("chunk1"))
	chunk2 := NewChunk(ids.GenerateTestNodeID(), now.Add(10*time.Minute).UnixMilli(), []byte("chunk2"))
	chunk3 := NewChunk(ids.GenerateTestNodeID(), now.Add(15*time.Minute).UnixMilli(), []byte("chunk3"))

	// Add chunks to store
	r.NoError(store.putPendingChunk(chunk1))
	r.NoError(store.putPendingChunk(chunk2))
	r.NoError(store.putPendingChunk(chunk3))

	// Verify chunks are retrievable and identical
	retrievedChunk1, ok := store.getPendingChunk(chunk1.id)
	r.True(ok)
	r.Equal(chunk1, retrievedChunk1)

	retrievedChunk2, ok := store.getPendingChunk(chunk2.id)
	r.True(ok)
	r.Equal(chunk2, retrievedChunk2)

	retrievedChunk3, ok := store.getPendingChunk(chunk3.id)
	r.True(ok)
	r.Equal(chunk3, retrievedChunk3)

	// Set min timestamp to exactly chunk2's timestamp
	r.NoError(store.setMin(now.Add(10*time.Minute).UnixMilli(), nil)) // XXX

	// Verify chunk1 is no longer present
	_, ok = store.getPendingChunk(chunk1.id)
	r.False(ok)

	// Verify chunks 2 and 3 are still present
	// Note: setMin's expected behavior is to remove all chunks with expiry strictly less than
	// the minimum timestamp.
	retrievedChunk2, ok = store.getPendingChunk(chunk2.id)
	r.True(ok)
	r.Equal(chunk2, retrievedChunk2)

	retrievedChunk3, ok = store.getPendingChunk(chunk3.id)
	r.True(ok)
	r.Equal(chunk3, retrievedChunk3)
}

func TestPendingChunkStore_Restart(t *testing.T) {
	r := require.New(t)

	db := memdb.New()
	ruleFactory := newRuleFactory(100, 60000)
	store, err := newPendingChunkStore(db, ruleFactory)
	r.NoError(err)
	r.NotNil(store)

	now := time.Now()
	chunk1 := NewChunk(ids.GenerateTestNodeID(), now.Add(5*time.Minute).UnixMilli(), []byte("chunk1"))
	chunk2 := NewChunk(ids.GenerateTestNodeID(), now.Add(10*time.Minute).UnixMilli(), []byte("chunk2"))

	// Add chunks to store
	r.NoError(store.putPendingChunk(chunk1))
	r.NoError(store.putPendingChunk(chunk2))

	// Verify chunks are retrievable and identical
	retrievedChunk1, ok := store.getPendingChunk(chunk1.id)
	r.True(ok)
	r.Equal(chunk1, retrievedChunk1)

	retrievedChunk2, ok := store.getPendingChunk(chunk2.id)
	r.True(ok)
	r.Equal(chunk2, retrievedChunk2)

	// Set min timestamp between chunks
	r.NoError(store.setMin(now.Add(7*time.Minute).UnixMilli(), nil)) // XXX

	// Create new instance of store with same DB
	store, err = newPendingChunkStore(db, ruleFactory)
	r.NoError(err)
	r.NotNil(store)

	// Verify chunk1 is no longer present
	_, ok = store.getPendingChunk(chunk1.id)
	r.False(ok)

	// Verify chunk2 is still present and identical
	retrievedChunk2, ok = store.getPendingChunk(chunk2.id)
	r.True(ok)
	r.Equal(chunk2, retrievedChunk2)
}

func TestPendingChunkStore_MaxBandwidth(t *testing.T) {
	r := require.New(t)

	db := memdb.New()
	ruleFactory := newRuleFactory(500, 60000) // 500 byte max bandwidth
	store, err := newPendingChunkStore(db, ruleFactory)
	r.NoError(err)
	r.NotNil(store)

	now := time.Now()
	validatorID := ids.GenerateTestNodeID()
	chunk1 := NewChunk(validatorID, now.Add(5*time.Minute).UnixMilli(), make([]byte, 10))
	chunk2 := NewChunk(validatorID, now.Add(10*time.Minute).UnixMilli(), make([]byte, 500))

	// First chunk should be accepted
	r.NoError(store.putPendingChunk(chunk1))

	// Second chunk should fail due to exceeding max bandwidth
	err = store.putPendingChunk(chunk2)
	r.ErrorIs(err, errExceedsPendingBandwidthLimit)
}

type rules struct {
	minBlockGap                     int64
	minEmptyBlockGap                int64
	maxPendingBandwidthPerValidator uint64
	validityWindow                  int64
	networkID                       uint32
	subnetID                        ids.ID
	chainID                         ids.ID
	quorumNum                       uint64
	quorumDen                       uint64
}

func (r *rules) GetMinBlockGap() int64 {
	return r.minBlockGap
}

func (r *rules) GetMinEmptyBlockGap() int64 {
	return r.minEmptyBlockGap
}

func (r *rules) GetMaxPendingBandwidthPerValidator() uint64 {
	return r.maxPendingBandwidthPerValidator
}

func (r *rules) GetValidityWindow() int64 {
	return r.validityWindow
}

func (r *rules) GetNetworkID() uint32 {
	return r.networkID
}

func (r *rules) GetSubnetID() ids.ID {
	return r.subnetID
}

func (r *rules) GetChainID() ids.ID {
	return r.chainID
}

func (r *rules) GetQuorumNum() uint64 {
	return r.quorumNum
}

func (r *rules) GetQuorumDen() uint64 {
	return r.quorumDen
}

type ruleFactory struct {
	rules *rules
}

func newRuleFactory(
	maxPendingBandwidthPerValidator uint64,
	validityWindow int64,
) *ruleFactory {
	return &ruleFactory{
		rules: &rules{
			maxPendingBandwidthPerValidator: maxPendingBandwidthPerValidator,
			validityWindow:                  validityWindow,
			// TODO: fill remain
		},
	}
}

func (rf *ruleFactory) GetRules(timestamp int64) Rules {
	return rf.rules
}
