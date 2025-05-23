// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

const (
	metadataByte byte = iota
	pendingByte
	acceptedByte

	minSlotByte byte = 0x00

	chunkKeySize                         = 1 + consts.Uint64Len + ids.IDLen
	validityWindowTimestampDivisor int64 = 1 // TODO: make this divisor configurable
)

var minSlotKey []byte = []byte{metadataByte, minSlotByte}

var (
	ErrChunkProducerNotValidator = errors.New("chunk producer is not in the validator set")
	ErrInvalidChunkCertificate   = errors.New("invalid chunk certificate")
	ErrChunkRateLimitSurpassed   = errors.New("chunk rate limit surpassed")
)

type Verifier[T Tx] interface {
	Verify(chunk Chunk[T]) error
	SetMin(updatedMin int64)
	VerifyCertificate(ctx context.Context, chunkCert *ChunkCertificate) error
}

var _ Verifier[Tx] = (*ChunkVerifier[Tx])(nil)

type ChunkVerifier[T Tx] struct {
	chainState  ChainState
	min         int64
	ruleFactory RuleFactory
}

func NewChunkVerifier[T Tx](chainState ChainState, ruleFactory RuleFactory) *ChunkVerifier[T] {
	verifier := &ChunkVerifier[T]{
		chainState:  chainState,
		ruleFactory: ruleFactory,
	}
	return verifier
}

func (c *ChunkVerifier[T]) SetMin(updatedMin int64) {
	c.min = updatedMin
}

func (c ChunkVerifier[T]) Verify(chunk Chunk[T]) error {
	// check if the expiry of this chunk isn't in the past or too far into the future.
	rules := c.ruleFactory.GetRules(c.min)
	validityWindowDuration := rules.GetValidityWindow()
	if err := validitywindow.VerifyTimestamp(chunk.Expiry, c.min, validityWindowTimestampDivisor, validityWindowDuration); err != nil {
		return err
	}

	// check if the producer was expected to produce this chunk.
	isValidator, err := c.chainState.IsNodeValidator(context.TODO(), chunk.UnsignedChunk.Producer, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to test whether a node belongs to the validator set during chunk verification", err)
	}
	if !isValidator {
		// the producer of this chunk isn't in the validator set.
		return fmt.Errorf("%w: producer node id %v", ErrChunkProducerNotValidator, chunk.UnsignedChunk.Producer)
	}

	return chunk.Verify(c.chainState.GetNetworkID(), c.chainState.GetChainID())
}

func (c ChunkVerifier[T]) VerifyCertificate(ctx context.Context, chunkCert *ChunkCertificate) error {
	err := chunkCert.Verify(
		ctx,
		c.chainState,
	)
	if err != nil {
		return fmt.Errorf("unable to verify chunk certificate: %w", err)
	}
	return nil
}

type StoredChunkSignature[T Tx] struct {
	Chunk Chunk[T]
	Cert  *ChunkCertificate
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
	pendingChunkMap map[ids.ID]*StoredChunkSignature[T]

	// pendingChunksSizes map a chunk producer to the total size of storage being used for it's pending chunks.
	pendingChunksSizes map[ids.NodeID]uint64

	ruleFactory RuleFactory
}

func NewChunkStorage[T Tx](
	verifier Verifier[T],
	db database.Database,
	ruleFactory RuleFactory,
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
		minimumExpiry:      minSlot,
		chunkEMap:          emap.NewEMap[emapChunk[T]](),
		pendingChunkMap:    make(map[ids.ID]*StoredChunkSignature[T]),
		pendingChunksSizes: make(map[ids.NodeID]uint64),
		chunkDB:            db,
		verifier:           verifier,
		ruleFactory:        ruleFactory,
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
		s.chunkEMap.Add([]emapChunk[T]{{chunk: chunk}})
		storedChunkSig := &StoredChunkSignature[T]{Chunk: chunk}
		s.pendingChunkMap[chunk.id] = storedChunkSig
		s.pendingChunksSizes[chunk.Producer] += uint64(len(chunk.bytes))
	}

	if err := iter.Error(); err != nil {
		return fmt.Errorf("failed to initialize storage due to iterator error: %w", err)
	}
	return nil
}

// AddLocalChunkWithCert adds a chunk to storage with the local signature share and aggregated certificate
// Assumes caller has already verified this does not add a duplicate chunk
func (s *ChunkStorage[T]) AddLocalChunkWithCert(c Chunk[T], cert *ChunkCertificate) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.putVerifiedChunk(c, cert)
}

// SetChunkCert sets the chunk certificate for the given chunkID
// Assumes the caller would call this function with a valid cert.
func (s *ChunkStorage[T]) SetChunkCert(ctx context.Context, chunkID ids.ID, cert *ChunkCertificate) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	storedChunk, ok := s.pendingChunkMap[chunkID]
	if !ok {
		return fmt.Errorf("failed to store cert for non-existent chunk: %s", chunkID)
	}

	if err := s.verifier.VerifyCertificate(ctx, cert); err != nil {
		return fmt.Errorf("failed to store invalid cert for chunk %s : %w", chunkID, err)
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
// Assumes caller has already verified this does not add a duplicate chunk
// Assumes that if the given chunk is a pending chunk, it would not surpass the producer's rate limit.
func (s *ChunkStorage[T]) VerifyRemoteChunk(c Chunk[T]) (*warp.BitSetSignature, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	chunkCertInfo, ok := s.pendingChunkMap[c.id]
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

// putVerifiedChunk assumes that the given chunk is guaranteed not to surpass the producer's rate limit.
// The rate limit is being checked via a call to CheckRateLimit from BuildChunk (for locally generated chunks)
// and ChunkSignatureRequestVerifier.Verify for incoming chunk signature requests.
func (s *ChunkStorage[T]) putVerifiedChunk(c Chunk[T], cert *ChunkCertificate) error {
	if err := s.chunkDB.Put(pendingChunkKey(c.Expiry, c.id), c.bytes); err != nil {
		return err
	}
	s.chunkEMap.Add([]emapChunk[T]{{chunk: c}})

	if chunkCert, ok := s.pendingChunkMap[c.id]; ok {
		if cert != nil {
			chunkCert.Cert = cert
		}
		return nil
	}
	chunkCert := &StoredChunkSignature[T]{Chunk: c, Cert: cert}
	s.pendingChunkMap[c.id] = chunkCert
	s.pendingChunksSizes[c.Producer] += uint64(len(c.bytes))

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
		chunk, ok := s.pendingChunkMap[saveChunkID]
		if !ok {
			return fmt.Errorf("failed to save chunk %s", saveChunkID)
		}
		if err := batch.Put(acceptedChunkKey(chunk.Chunk.Expiry, chunk.Chunk.id), chunk.Chunk.bytes); err != nil {
			return fmt.Errorf("failed to save chunk %s: %w", saveChunkID, err)
		}
		s.discardPendingChunk(saveChunkID)
	}
	expiredChunks := s.chunkEMap.SetMin(updatedMin)
	for _, chunkID := range expiredChunks {
		chunk, ok := s.pendingChunkMap[chunkID]
		if !ok {
			continue
		}
		s.discardPendingChunk(chunkID)
		// TODO: switch to using DeleteRange(nil, pendingChunkKey(updatedMin, ids.Empty)) after
		// merging main
		if err := batch.Delete(pendingChunkKey(chunk.Chunk.Expiry, chunk.Chunk.id)); err != nil {
			return err
		}
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to write SetMin batch: %w", err)
	}
	s.verifier.SetMin(updatedMin)
	return nil
}

// discardPendingChunk removes the given chunkID from the
// pending chunk map as well as from the pending chunks producers map.
func (s *ChunkStorage[T]) discardPendingChunk(chunkID ids.ID) {
	chunk, ok := s.pendingChunkMap[chunkID]
	if !ok {
		return
	}
	delete(s.pendingChunkMap, chunkID)
	s.pendingChunksSizes[chunk.Chunk.Producer] -= uint64(len(chunk.Chunk.bytes))
	if s.pendingChunksSizes[chunk.Chunk.Producer] == 0 {
		delete(s.pendingChunksSizes, chunk.Chunk.Producer)
	}
}

// GatherChunkCerts provides a slice of chunk certificates to build
// a chunk based block.
// TODO: switch from returning random chunk certs to ordered by expiry
func (s *ChunkStorage[T]) GatherChunkCerts() []*ChunkCertificate {
	s.lock.RLock()
	defer s.lock.RUnlock()

	chunkCerts := make([]*ChunkCertificate, 0, len(s.pendingChunkMap))
	for _, chunk := range s.pendingChunkMap {
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
func (s *ChunkStorage[T]) GetChunkBytes(expiry int64, chunkID ids.ID) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	chunk, ok := s.pendingChunkMap[chunkID]
	if ok {
		return chunk.Chunk.bytes, nil
	}

	chunkBytes, err := s.chunkDB.Get(acceptedChunkKey(expiry, chunkID))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accepted chunk bytes for %s: %w", chunkID, err)
	}
	return chunkBytes, nil
}

func (s *ChunkStorage[T]) CheckRateLimit(chunk Chunk[T]) error {
	weightLimit := s.ruleFactory.GetRules(chunk.Expiry).GetMaxAccumulatedProducerChunkWeight()

	if uint64(len(chunk.bytes))+s.pendingChunksSizes[chunk.Producer] > weightLimit {
		return ErrChunkRateLimitSurpassed
	}
	return nil
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
