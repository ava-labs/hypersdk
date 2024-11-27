// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/emap"
)

const (
	metadataByte byte = iota
	pendingByte
	acceptedByte

	minSlotByte byte = 0x00

	chunkKeySize = 1 + consts.Uint64Len + ids.IDLen
)

var minSlotKey []byte = []byte{metadataByte, minSlotByte}

type Verifier[T Tx] interface {
	Verify(chunk Chunk[T]) error
}

var _ Verifier[Tx] = (*NoVerifier[Tx])(nil)

type NoVerifier[T Tx] struct{}

func (NoVerifier[T]) Verify(Chunk[T]) error {
	return nil
}

type StoredChunkSignature[T Tx] struct {
	Chunk     Chunk[T]
	Cert      *ChunkCertificate[T]
	Available bool
}

// ChunkStorage provides chunk, signature share, and chunk certificate storage
//
// Note: we only require chunk persistence until it has either been included
// or expired.
//
// We do not require persistence of chunk certificates.
// If a valid chunk certificate is included in a block, we already have it.
// If a valid chunk certificate is dropped, the network may drop the chunk.
type ChunkStorage[T Tx] struct {
	verifier Verifier[T]

	lock sync.RWMutex
	// Chunk storage
	chunkEMap     *emap.EMap[emapChunk[T]]
	minimumExpiry int64
	// pendingByte | slot | chunkID -> chunkBytes
	// acceptedByte | slot | chunkID -> chunkBytes
	chunkDB database.Database

	// TODO do we need caching
	// Chunk + signature + cert
	chunkMap map[ids.ID]*StoredChunkSignature[T]
}

func NewChunkStorage[T Tx](
	verifier Verifier[T],
	db database.Database,
) (*ChunkStorage[T], error) {
	minSlot := int64(0)
	minSlotBytes, err := db.Get(minSlotKey)
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}

	if err == nil {
		minSlotUint64, err := database.ParseUInt64(minSlotBytes)
		if err != nil {
			return nil, err
		}
		minSlot = int64(minSlotUint64)
	}

	storage := &ChunkStorage[T]{
		minimumExpiry: minSlot,
		chunkEMap:     emap.NewEMap[emapChunk[T]](),
		chunkMap:      make(map[ids.ID]*StoredChunkSignature[T]),
		chunkDB:       db,
		verifier:      verifier,
	}
	return storage, storage.init()
}

func (s *ChunkStorage[T]) init() error {
	iter := s.chunkDB.NewIteratorWithPrefix([]byte{pendingByte})
	defer iter.Release()

	for iter.Next() {
		chunk, err := ParseChunk[T](iter.Value())
		if err != nil {
			slot, chunkID, keyParsingErr := parsePendingChunkKey(iter.Key())
			if keyParsingErr != nil {
				return err
			}
			return fmt.Errorf("failed to parse chunk %s at slot %d", chunkID, slot)
		}
		_, err = s.VerifyRemoteChunk(chunk)
		if err != nil {
			return err
		}
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("failed to initialize storage due to iterator error: %w", err)
	}
	return nil
}

// AddLocalChunkWithCert adds a chunk to storage with the local signature share and aggregated certificate
func (s *ChunkStorage[T]) AddLocalChunkWithCert(c Chunk[T], cert *ChunkCertificate[T]) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.putVerifiedChunk(c, cert)
}

// SetChunkCert sets the chunk certificate for the given chunkID
// Assumes the caller has already verified the cert references the provided chunkID
func (s *ChunkStorage[T]) SetChunkCert(chunkID ids.ID, cert *ChunkCertificate[T]) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	storedChunk, ok := s.chunkMap[chunkID]
	if !ok {
		return fmt.Errorf("failed to store cert for non-existent chunk: %s", chunkID)
	}
	storedChunk.Cert = cert
	return nil
}

// VerifyRemoteChunk will:
// 1. Check the cache
// 2. Verify the chunk
// 3. Generate a local signature share and store it in memory
// 4. Return the local signature share
// TODO refactor and merge with AddLocalChunkWithCert
func (s *ChunkStorage[T]) VerifyRemoteChunk(c Chunk[T]) (*warp.BitSetSignature, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	chunkCertInfo, ok := s.chunkMap[c.id]
	if ok {
		return chunkCertInfo.Cert.Signature, nil
	}
	if err := s.verifier.Verify(c); err != nil {
		return nil, err
	}
	if err := s.putVerifiedChunk(c, nil); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *ChunkStorage[T]) putVerifiedChunk(c Chunk[T], cert *ChunkCertificate[T]) error {
	if err := s.chunkDB.Put(pendingChunkKey(c.Expiry, c.id), c.bytes); err != nil {
		return err
	}
	s.chunkEMap.Add([]emapChunk[T]{{chunk: c}})

	chunkCert := &StoredChunkSignature[T]{
		Chunk: c,
		Cert:  cert,
	}
	s.chunkMap[c.id] = chunkCert
	return nil
}

// TODO need to call this to expire chunks in server
// SetMin sets the minimum timestamp on the expiring storage and marks the chunks that
// must be saved, which would otherwise expire.
func (s *ChunkStorage[T]) SetMin(updatedMin int64, saveChunks []ids.ID) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.minimumExpiry = updatedMin
	minSlotBytes := make([]byte, consts.Uint64Len)
	binary.BigEndian.PutUint64(minSlotBytes, uint64(s.minimumExpiry))
	batch := s.chunkDB.NewBatch()
	if err := batch.Put(minSlotKey, minSlotBytes); err != nil {
		return fmt.Errorf("failed to update persistent min slot: %w", err)
	}
	for _, saveChunkID := range saveChunks {
		chunk, ok := s.chunkMap[saveChunkID]
		if !ok {
			return fmt.Errorf("failed to save chunk %s", saveChunkID)
		}
		chunk.Available = true
		if err := batch.Put(acceptedChunkKey(chunk.Chunk.Expiry, chunk.Chunk.id), chunk.Chunk.bytes); err != nil {
			return fmt.Errorf("failed to save chunk %s: %w", saveChunkID, err)
		}
	}
	expiredChunks := s.chunkEMap.SetMin(updatedMin)
	for _, chunkID := range expiredChunks {
		chunk, ok := s.chunkMap[chunkID]
		if !ok {
			continue
		}
		delete(s.chunkMap, chunkID)
		// TODO: switch to using DeleteRange(nil, pendingChunkKey(updatedMin, ids.Empty)) after
		// merging main
		if err := batch.Delete(pendingChunkKey(chunk.Chunk.Expiry, chunk.Chunk.id)); err != nil {
			return err
		}
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write SetMin batch: %w", err)
	}
	return nil
}

// GatherChunkCerts provides a slice of chunk certificates to build
// a chunk based block.
// TODO: switch from returning random chunk certs to ordered by expiry
func (s *ChunkStorage[T]) GatherChunkCerts() []*ChunkCertificate[T] {
	s.lock.RLock()
	defer s.lock.RUnlock()

	chunkCerts := make([]*ChunkCertificate[T], 0, len(s.chunkMap))
	for _, chunk := range s.chunkMap {
		if chunk.Cert == nil {
			continue
		}
		chunkCerts = append(chunkCerts, chunk.Cert)
	}
	return chunkCerts
}

// GetChunkBytes returns the corresponding chunk bytes of the requested chunk
// Both the slot and chunkID must be provided to create the relevant DB key, which
// includes the slot to create a more sequential DB workload.
func (s *ChunkStorage[T]) GetChunkBytes(expiry int64, chunkID ids.ID) ([]byte, bool, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	chunk, ok := s.chunkMap[chunkID]
	if ok {
		return chunk.Chunk.bytes, chunk.Available, nil
	}

	if expiry < s.minimumExpiry { // Chunk can only be in accepted section of the DB
		chunkBytes, err := s.chunkDB.Get(acceptedChunkKey(expiry, chunkID))
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch accepted chunk bytes for %s: %w", chunkID, err)
		}
		return chunkBytes, true, nil
	}

	chunkBytes, err := s.chunkDB.Get(pendingChunkKey(expiry, chunkID))
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch chunk bytes for %s: %w", chunkID, err)
	}
	return chunkBytes, false, nil
}

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

func acceptedChunkKey(slot int64, chunkID ids.ID) []byte {
	return createChunkKey(acceptedByte, slot, chunkID)
}

var _ emap.Item = (*emapChunk[Tx])(nil)

type emapChunk[T Tx] struct {
	chunk Chunk[T]
}

func (e emapChunk[_]) GetID() ids.ID {
	return e.chunk.id
}

func (e emapChunk[_]) GetExpiry() int64 {
	return e.chunk.Expiry
}
