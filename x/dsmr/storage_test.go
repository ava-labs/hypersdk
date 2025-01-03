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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"
)

var errInvalidTestItem = errors.New("invalid test item")

type testVerifier[T Tx[T]] struct {
	correctIDs set.Set[ids.ID]
}

func (t testVerifier[T]) Verify(chunk Chunk[T]) error {
	if t.correctIDs.Contains(chunk.id) {
		return nil
	}
	return fmt.Errorf("%w: %s", errInvalidTestItem, chunk.id)
}

func createTestStorage(t *testing.T, numValidChunks, numInvalidChunks int) (
	*ChunkStorage[*dsmrtest.Tx],
	[]Chunk[*dsmrtest.Tx],
	[]Chunk[*dsmrtest.Tx],
	func() *ChunkStorage[*dsmrtest.Tx],
) {
	require := require.New(t)

	validChunks := make([]Chunk[*dsmrtest.Tx], 0, numValidChunks)
	for i := 1; i <= numValidChunks; i++ { // emap does not support expiry of 0
		chunk := newChunk(
			UnsignedChunk[*dsmrtest.Tx]{
				Producer:    ids.EmptyNodeID,
				Beneficiary: codec.Address{},
				Expiry:      time.Now().Unix(),
				Txs:         []*dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
			},
			[48]byte{},
			[96]byte{},
		)
		validChunks = append(validChunks, chunk)
	}

	invalidChunks := make([]Chunk[*dsmrtest.Tx], 0, numInvalidChunks)
	for i := 1; i <= numInvalidChunks; i++ { // emap does not support expiry of 0
		chunk := newChunk(
			UnsignedChunk[*dsmrtest.Tx]{
				Producer:    ids.EmptyNodeID,
				Beneficiary: codec.Address{},
				Expiry:      time.Now().Unix(),
				Txs:         []*dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
			},
			[48]byte{},
			[96]byte{},
		)
		invalidChunks = append(invalidChunks, chunk)
	}

	tempDir := t.TempDir()

	db, err := pebble.New(tempDir, pebble.NewDefaultConfig(), prometheus.NewRegistry())
	require.NoError(err)
	validChunkIDs := make([]ids.ID, 0, numValidChunks)
	for _, chunk := range validChunks {
		validChunkIDs = append(validChunkIDs, chunk.id)
	}

	testVerifier := testVerifier[*dsmrtest.Tx]{correctIDs: set.Of(validChunkIDs...)}
	storage, err := NewChunkStorage[*dsmrtest.Tx](
		testVerifier,
		db,
	)
	require.NoError(err)

	restart := func() *ChunkStorage[*dsmrtest.Tx] {
		require.NoError(db.Close())
		db, err = pebble.New(tempDir, pebble.NewDefaultConfig(), prometheus.NewRegistry())
		require.NoError(err)

		storage, err := NewChunkStorage[*dsmrtest.Tx](
			testVerifier,
			db,
		)
		require.NoError(err)
		return storage
	}
	return storage, validChunks, invalidChunks, restart
}

func TestStoreAndSaveValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.NoError(err)

	foundChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)

	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
	}
	require.NoError(storage.SetChunkCert(chunk.id, chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, []ids.ID{chunk.id}))

	foundAcceptedChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.NoError(err)

	foundChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)

	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
	}
	require.NoError(storage.SetChunkCert(chunk.id, chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, nil))

	_, _, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.ErrorIs(err, database.ErrNotFound)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreInvalidChunk(t *testing.T) {
	require := require.New(t)

	storage, _, invalidChunks, _ := createTestStorage(t, 0, 1)
	chunk := invalidChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.ErrorIs(err, errInvalidTestItem)

	_, _, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.ErrorIs(err, database.ErrNotFound)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndSaveLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
	}

	require.NoError(storage.AddLocalChunkWithCert(chunk, chunkCert))

	foundChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, []ids.ID{chunk.id}))

	foundAcceptedChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
	}

	require.NoError(storage.AddLocalChunkWithCert(chunk, chunkCert))

	foundChunkBytes, _, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, nil))

	_, _, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.ErrorIs(err, database.ErrNotFound)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestRestartSavedChunks(t *testing.T) {
	t.Skip("TODO")
	require := require.New(t)

	// Test persistent chunk storage for each of the following cases:
	// 1. Accepted local chunk
	// 2. Accepted remote chunk
	// 3. Expired local chunk
	// 4. Expired remote chunk
	// 5. Pending local chunk
	// 6. Pending remote chunk
	numChunks := 6
	storage, validChunks, _, restart := createTestStorage(t, numChunks, 0)
	chunkCerts := make([]*ChunkCertificate, 0, numChunks)
	for _, chunk := range validChunks {
		chunkCert := &ChunkCertificate{
			ChunkReference: ChunkReference{
				ChunkID:  chunk.id,
				Producer: chunk.Producer,
				Expiry:   chunk.Expiry,
			},
		}
		chunkCerts = append(chunkCerts, chunkCert)
	}

	// Case 1
	require.NoError(storage.AddLocalChunkWithCert(validChunks[0], chunkCerts[0]))

	// Case 2
	_, err := storage.VerifyRemoteChunk(validChunks[1])
	require.NoError(err)
	require.NoError(storage.SetChunkCert(validChunks[1].id, chunkCerts[1]))

	// Case 3
	require.NoError(storage.AddLocalChunkWithCert(validChunks[2], chunkCerts[2]))

	// Case 4
	_, err = storage.VerifyRemoteChunk(validChunks[3])
	require.NoError(err)

	// Case 5
	require.NoError(storage.AddLocalChunkWithCert(validChunks[4], chunkCerts[4]))

	// Case 6
	_, err = storage.VerifyRemoteChunk(validChunks[5])
	require.NoError(err)
	require.NoError(storage.SetChunkCert(validChunks[5].id, chunkCerts[5]))

	// Set the minimum to 5 and mark cases 1 and 2 as saved
	require.NoError(storage.SetMin(5, []ids.ID{
		validChunks[0].id,
		validChunks[1].id,
	}))

	confirmChunkStorage := func(storage *ChunkStorage[*dsmrtest.Tx]) {
		// Confirm we can fetch the chunk bytes for the accepted and pending chunks
		for i, expectedChunk := range []Chunk[*dsmrtest.Tx]{
			validChunks[0],
			validChunks[1],
			validChunks[4],
			validChunks[5],
		} {
			foundChunkBytes, _, err := storage.GetChunkBytes(expectedChunk.Expiry, expectedChunk.id)
			require.NoError(err, i)
			require.Equal(expectedChunk.bytes, foundChunkBytes, i)
		}

		// Confirm the expired chunks are garbage collected
		for _, expectedChunk := range []Chunk[*dsmrtest.Tx]{
			validChunks[2],
			validChunks[3],
		} {
			_, _, err = storage.GetChunkBytes(expectedChunk.Expiry, expectedChunk.id)
			require.ErrorIs(err, database.ErrNotFound)
		}
	}
	confirmChunkStorage(storage)
	storage = restart()
	confirmChunkStorage(storage)
}
