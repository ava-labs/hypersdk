package dsmr

import (
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type ChunkWithSignature[T Tx] struct {
	Chunk     *Chunk[T]
	Signature ChunkSignatureShare
}

type ChunkWithAggregateSignature[T Tx] struct {
	Chunk     *Chunk[T]
	Signature ChunkSignature
}

type Storage[T Tx] struct {
	chunkEMap    *emap.EMap[*Chunk[T]]
	storedChunks map[ids.ID]*ChunkWithSignature[T]

	// chunkID -> chunkBytes
	// TODO: consider switch to slot | chunkID -> chunkBytes
	chunkDB database.Database
}

func NewStorage[T Tx](db database.Database) *Storage[T] {
	return &Storage[T]{
		chunkEMap:    emap.NewEMap[*Chunk[T]](),
		storedChunks: make(map[ids.ID]*ChunkWithSignature[T]),
		chunkDB:      db,
	}
}

func (s *Storage[T]) VerifyChunk(c *Chunk[T]) (ChunkSignatureShare, error) {
	chunkWithSignature, ok := s.storedChunks[c.ID()]
	if ok {
		return chunkWithSignature.Signature, nil
	}
	// TODO: verify remote chunk
	// TODO: add persistent storage
	s.chunkEMap.Add([]*Chunk[T]{c})

	chunkWithSignature = &ChunkWithSignature[T]{
		Chunk:     c,
		Signature: ChunkSignatureShare{},
	}
	s.storedChunks[c.ID()] = chunkWithSignature

	// TODO: return a real signature
	return chunkWithSignature.Signature, nil
}

func (s *Storage[T]) SetMin(updatedMin int64, saveChunks []ids.ID) error {
	for _, saveChunkID := range saveChunks {
		chunk, ok := s.storedChunks[saveChunkID]
		if !ok {
			return fmt.Errorf("failed to save chunk %s", saveChunkID)
		}
		if err := s.chunkDB.Put(saveChunkID[:], chunk.Chunk.Bytes()); err != nil {
			return fmt.Errorf("failed to save chunk %s: %w", saveChunkID, err)
		}
	}
	expiredChunks := s.chunkEMap.SetMin(updatedMin)
	for _, chunkID := range expiredChunks {
		delete(s.storedChunks, chunkID)
	}
	return nil
}

func (s *Storage[T]) GetChunkBytes(chunkID ids.ID) ([]byte, error) {
	chunk, ok := s.storedChunks[chunkID]
	if ok {
		return chunk.Chunk.Bytes(), nil
	}

	chunkBytes, err := s.chunkDB.Get(chunkID[:])
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunk bytes for %s: %w", chunkID, err)
	}
	return chunkBytes, nil
}
