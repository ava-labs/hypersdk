// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/stretchr/testify/require"
)

var errInvalidTestItem = errors.New("invalid test item")

var _ Verifier[testContextProvider, IDer] = testVerifier[IDer]{}

type testContextProvider struct{}

func (t testContextProvider) Context() testContextProvider {
	return t
}

type testVerifier[T IDer] struct {
	correctIDs set.Set[ids.ID]
}

type IDer interface {
	ID() ids.ID
}

type testTx struct {
	ID     ids.ID `serialize:"true"`
	Expiry int64  `serialize:"true"`
}

func (t testTx) GetID() ids.ID        { return t.ID }
func (t testTx) GetExpiry() time.Time { return time.Unix(0, t.Expiry) }

func (t testVerifier[T]) Verify(_ testContextProvider, item T) error {
	if t.correctIDs.Contains(item.ID()) {
		return nil
	}
	return fmt.Errorf("%w: %s", errInvalidTestItem, item.ID())
}

func createTestStorage(t *testing.T, numValidChunks, numInvalidChunks int) (
	*Storage[testContextProvider, testTx],
	[]Chunk[testTx],
	[]Chunk[testTx],
) {
	require := require.New(t)

	validChunks := make([]Chunk[testTx], 0, numValidChunks)
	for i := 1; i <= numValidChunks; i++ { // emap does not support expiry of 0
		chunk, err := NewChunk([]testTx{
			{ID: ids.GenerateTestID(), Expiry: 1_000_000},
		}, int64(i))
		require.NoError(err)
		validChunks = append(validChunks, chunk)
	}

	invalidChunks := make([]Chunk[testTx], 0, numInvalidChunks)
	for i := 1; i <= numInvalidChunks; i++ { // emap does not support expiry of 0
		chunk, err := NewChunk([]testTx{
			{ID: ids.GenerateTestID(), Expiry: 1_000_000},
		}, int64(i))
		require.NoError(err)
		invalidChunks = append(invalidChunks, chunk)
	}

	tempDir := t.TempDir()

	db, _, err := pebble.New(tempDir, pebble.NewDefaultConfig())
	require.NoError(err)
	validChunkIDs := make([]ids.ID, 0, numValidChunks)
	for _, chunk := range validChunks {
		validChunkIDs = append(validChunkIDs, chunk.ID())
	}

	testVerifier := testVerifier[*Chunk[testTx]]{correctIDs: set.Of(validChunkIDs...)}
	storage, err := NewStorage(
		testContextProvider{},
		testVerifier,
		db.(*pebble.Database),
	)
	require.NoError(err)
	return storage, validChunks, invalidChunks
}

func TestStoreAndSaveValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(&chunk)
	require.NoError(err)

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)

	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.ID(),
		Slot:      chunk.Slot,
		Signature: ChunkSignature{},
	}
	require.NoError(storage.SetChunkCert(chunk.ID(), chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Slot+1, []ids.ID{chunk.ID()}))

	foundAcceptedChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(&chunk)
	require.NoError(err)

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)

	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.ID(),
		Slot:      chunk.Slot,
		Signature: ChunkSignature{},
	}
	require.NoError(storage.SetChunkCert(chunk.ID(), chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Slot+1, nil))

	_, err = storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.ErrorIs(err, database.ErrNotFound)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreInvalidChunk(t *testing.T) {
	require := require.New(t)

	storage, _, invalidChunks := createTestStorage(t, 0, 1)
	chunk := invalidChunks[0]

	_, err := storage.VerifyRemoteChunk(&chunk)
	require.ErrorIs(err, errInvalidTestItem)

	_, err = storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.ErrorIs(err, database.ErrNotFound)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndSaveLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.ID(),
		Slot:      chunk.Slot,
		Signature: ChunkSignature{},
	}

	require.NoError(storage.AddLocalChunkWithCert(&chunk, chunkCert))

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Slot+1, []ids.ID{chunk.ID()}))

	foundAcceptedChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.ID(),
		Slot:      chunk.Slot,
		Signature: ChunkSignature{},
	}

	require.NoError(storage.AddLocalChunkWithCert(&chunk, chunkCert))

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.NoError(err)
	require.Equal(chunk.Bytes(), foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Slot+1, nil))

	_, err = storage.GetChunkBytes(chunk.Slot, chunk.ID())
	require.ErrorIs(err, database.ErrNotFound)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

// TODO:
// TestRestartSavedChunks
// Write interface / function signatures for p2p client/server w/ ACP-118 and chunk builder - getChunk, putChunk, getSignatureShare, putSignatureShare, putChunkCertificate
// Implement chunk block builder based off of storage - implement BuildBlock function that takes in storage decision: what ordering to use for adding chunks to a block?
// Implement chunk executor (fetch and re-assemble as needed) - given a chunk block, filter and create new block type with assembler interface
// Implement block re-assembler and executor - define block assembler and execute functions for existing chain.Block type and make into an interface
// Integrate into HyperSDK w/ ghost signature shares / certificates
// Inject Avalanche verification (Warp)
// Implement fortification
