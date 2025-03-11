// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/emap"
)

const (
	metadataByte byte = iota + 1
	pendingByte

	minTimestampByte byte = iota + 1

	chunkKeySize                         = 1 + consts.Uint64Len + ids.IDLen
	validityWindowTimestampDivisor int64 = 1 // TODO: make this divisor configurable
)

var minTimestampKey []byte = []byte{metadataByte, minTimestampByte}

var errExceedsPendingBandwidthLimit = errors.New("chunk exceeds bandwidth limit")

// pendingChunkStore provides the storage backend for chunks.
//
// To implement DSMR, we must guarantee any chunk that we've signed has been committed to disk.
// If correct nodes signed chunks without first committing them to disk, then the system could
// observe a threshold certificate for a chunk that cannot be retrieved.
// This ensures any chunk certificate with f + 1 signatures, guarantees at least one correct
// node has committed the data and the network can retrieve the data from at least one correct
// node.
//
// The chunk store additionally implements timestamp based garbage collection as blocks are accepted.
// When a block is accepted it advances the chain's timestamp, which may invalidate chunks we have
// stored. Once they can no longer be included onchain, we can safely garbage collect them.
//
// Moving accepted chunks from pending to a separate store is external to the pendingChunkStore.
type pendingChunkStore struct {
	lock sync.RWMutex

	ruleFactory RuleFactory
	db          database.Database
	// map of all committed pending chunkIDs -> unsigned chunks
	pendingChunks map[ids.ID]*UnsignedChunk

	// chunkEMap provides an in-memory heap of pending chunks to provide an efficient
	// way to garbage collect expired chunks that have not been accepted as they expire.
	chunkEMap *emap.EMap[eChunk]

	// pendingValidatorChunks tracks the total bandwidth of pending chunks per validator
	pendingValidatorChunks map[ids.NodeID]uint64

	minTimestamp int64
}

func newPendingChunkStore(
	db database.Database,
	ruleFactory RuleFactory,
) (*pendingChunkStore, error) {
	store := &pendingChunkStore{
		db:                     db,
		pendingChunks:          make(map[ids.ID]*UnsignedChunk),
		chunkEMap:              emap.NewEMap[eChunk](),
		pendingValidatorChunks: make(map[ids.NodeID]uint64),
		ruleFactory:            ruleFactory,
	}

	iter := store.db.NewIterator()
	for iter.Next() {
		unsignedChunkBytes := iter.Value()
		unsignedChunk, err := ParseUnsignedChunk(unsignedChunkBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pending chunk: %w", err)
		}
		store.pendingChunks[unsignedChunk.id] = unsignedChunk
		store.chunkEMap.Add([]eChunk{{
			chunkID: unsignedChunk.id,
			expiry:  unsignedChunk.Expiry,
		}})
		store.pendingValidatorChunks[unsignedChunk.Builder] += uint64(len(unsignedChunk.bytes))
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("failed to iterate pending chunk store: %w", err)
	}

	// Load the minimum timestamp from the database
	timestampBytes, err := store.db.Get(minTimestampKey)
	if err != nil && err != database.ErrNotFound {
		return nil, fmt.Errorf("failed to get min timestamp: %w", err)
	}
	if len(timestampBytes) != 0 {
		store.minTimestamp = int64(binary.BigEndian.Uint64(timestampBytes))
	}

	return store, nil
}

// setMin garbage collects expired pending chunks and updates the minimum slot stored on disk.
func (c *pendingChunkStore) setMin(updatedMin int64) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// garbage collect expired pending chunks
	c.minTimestamp = updatedMin
	expiredChunks := c.chunkEMap.SetMin(updatedMin)

	expiredChunkKeys := make([][]byte, len(expiredChunks))
	for i, expiredChunkID := range expiredChunks {
		pendingChunk := c.pendingChunks[expiredChunkID]
		delete(c.pendingChunks, expiredChunkID)
		c.pendingValidatorChunks[c.pendingChunks[expiredChunkID].Builder] -= uint64(len(c.pendingChunks[expiredChunkID].bytes))
		expiredChunkKeys[i] = pendingChunkKey(pendingChunk.Expiry, pendingChunk.id)
	}

	// Delete the expired chunks from disk.
	// Note: we can either delete the expired chunks one by one or
	// via DeleteRange using the updatedMin timestamp as the end key.
	batch := c.db.NewBatch()
	if err := batch.Put(minTimestampKey, binary.BigEndian.AppendUint64(nil, uint64(updatedMin))); err != nil {
		return fmt.Errorf("failed to write min timestamp %d to batch: %w", updatedMin, err)
	}
	for _, expiredChunkKey := range expiredChunkKeys {
		if err := batch.Delete(expiredChunkKey); err != nil {
			return fmt.Errorf("failed to delete expired chunk key %x: %w", expiredChunkKey, err)
		}
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write batch delete expired chunks up to timestamp %d: %w", updatedMin, err)
	}

	return nil
}

func (c *pendingChunkStore) putPendingChunk(unsignedChunk *UnsignedChunk) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	rules := c.ruleFactory.GetRules(c.minTimestamp)
	maxOutstandingBandwidth := rules.GetMaxPendingBandwidthPerValidator()
	pendingBytesFromValidator := c.pendingValidatorChunks[unsignedChunk.Builder]
	if pendingBytesFromValidator+uint64(len(unsignedChunk.bytes)) > maxOutstandingBandwidth {
		return fmt.Errorf(
			"%w (size = %d) from validator %s with outstanding bandwidth %d)",
			errExceedsPendingBandwidthLimit,
			len(unsignedChunk.bytes),
			unsignedChunk.Builder,
			maxOutstandingBandwidth,
		)
	}

	key := pendingChunkKey(unsignedChunk.Expiry, unsignedChunk.id)
	if err := c.db.Put(key, unsignedChunk.bytes); err != nil {
		return fmt.Errorf("failed to commit chunk %s to disk: %w", unsignedChunk.id, err)
	}

	c.pendingChunks[unsignedChunk.id] = unsignedChunk
	c.chunkEMap.Add([]eChunk{{
		chunkID: unsignedChunk.id,
		expiry:  unsignedChunk.Expiry,
	}})
	c.pendingValidatorChunks[unsignedChunk.Builder] += uint64(len(unsignedChunk.bytes))
	return nil
}

// getPendingChunk returns the pending chunk if available.
// Note: all pending chunks are stored in-memory, so this function never reads from disk.
func (c *pendingChunkStore) getPendingChunk(chunkID ids.ID) (*UnsignedChunk, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	pendingChunk, ok := c.pendingChunks[chunkID]
	return pendingChunk, ok
}

// Define helpers for creating and parsing database keys
// [prefix (pending/accepted) (1 byte) | slot (8 byte uint64) | chunkID (32 byte ID)]
func createChunkKey(prefix byte, slot int64, chunkID ids.ID) []byte {
	b := make([]byte, chunkKeySize)
	b[0] = prefix
	binary.BigEndian.PutUint64(b[1:1+consts.Uint64Len], uint64(slot))
	copy(b[1+consts.Uint64Len:], chunkID[:])
	return b
}

func parseChunkKey(key []byte) (prefix byte, slot int64, chunkID ids.ID, err error) {
	if len(key) != chunkKeySize {
		return 0, 0, ids.Empty, fmt.Errorf("unexpected chunk key size %d", len(key))
	}
	prefix = key[0]
	slot = int64(binary.BigEndian.Uint64(key[1 : 1+consts.Uint64Len]))
	copy(chunkID[:], key[1+consts.Uint64Len:])
	return prefix, slot, chunkID, nil
}

func pendingChunkKey(slot int64, chunkID ids.ID) []byte {
	return createChunkKey(pendingByte, slot, chunkID)
}

func parsePendingChunkKey(key []byte) (slot int64, chunkID ids.ID, err error) {
	prefix, slot, chunkID, err := parseChunkKey(key)
	if err != nil {
		return 0, ids.Empty, err
	}
	if prefix != pendingByte {
		return 0, ids.Empty, fmt.Errorf("unexpected pending chunk key prefix: %d", prefix)
	}
	return slot, chunkID, nil
}
