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
	func() *Storage[testContextProvider, testTx],
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
	pebbleDB := db.(*pebble.Database)

	testVerifier := testVerifier[*Chunk[testTx]]{correctIDs: set.Of(validChunkIDs...)}
	storage, err := NewStorage(
		testContextProvider{},
		testVerifier,
		pebbleDB,
	)
	require.NoError(err)

	restart := func() *Storage[testContextProvider, testTx] {
		require.NoError(pebbleDB.Close())
		db, _, err = pebble.New(tempDir, pebble.NewDefaultConfig())
		require.NoError(err)

		pebbleDB = db.(*pebble.Database)
		storage, err := NewStorage(
			testContextProvider{},
			testVerifier,
			pebbleDB,
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

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
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

	storage, _, invalidChunks, _ := createTestStorage(t, 0, 1)
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

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
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

	storage, validChunks, _, _ := createTestStorage(t, 1, 0)
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

func TestRestartSavedChunks(t *testing.T) {
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
			ChunkID:   chunk.ID(),
			Slot:      chunk.Slot,
			Signature: ChunkSignature{},
		}
		chunkCerts = append(chunkCerts, chunkCert)
	}

	// Case 1
	require.NoError(storage.AddLocalChunkWithCert(&validChunks[0], chunkCerts[0]))

	// Case 2
	_, err := storage.VerifyRemoteChunk(&validChunks[1])
	require.NoError(err)
	require.NoError(storage.SetChunkCert(validChunks[1].ID(), chunkCerts[1]))

	// Case 3
	require.NoError(storage.AddLocalChunkWithCert(&validChunks[2], chunkCerts[2]))

	// Case 4
	_, err = storage.VerifyRemoteChunk(&validChunks[3])
	require.NoError(err)

	// Case 5
	require.NoError(storage.AddLocalChunkWithCert(&validChunks[4], chunkCerts[4]))

	// Case 6
	_, err = storage.VerifyRemoteChunk(&validChunks[5])
	require.NoError(err)
	require.NoError(storage.SetChunkCert(validChunks[5].ID(), chunkCerts[5]))

	// Set the minimum to 5 and mark cases 1 and 2 as saved
	require.NoError(storage.SetMin(5, []ids.ID{
		validChunks[0].ID(),
		validChunks[1].ID(),
	}))

	confirmChunkStorage := func(storage *Storage[testContextProvider, testTx]) {
		// Confirm we can fetch the chunk bytes for the accepted and pending chunks
		for i, expectedChunk := range []Chunk[testTx]{
			validChunks[0],
			validChunks[1],
			validChunks[4],
			validChunks[5],
		} {
			foundChunkBytes, err := storage.GetChunkBytes(expectedChunk.Slot, expectedChunk.ID())
			require.NoError(err, i)
			require.Equal(expectedChunk.Bytes(), foundChunkBytes, i)
		}

		// Confirm the expired chunks are garbage collected
		for _, expectedChunk := range []Chunk[testTx]{
			validChunks[2],
			validChunks[3],
		} {
			_, err = storage.GetChunkBytes(expectedChunk.Slot, expectedChunk.ID())
			require.ErrorIs(err, database.ErrNotFound)
		}
	}
	confirmChunkStorage(storage)
	storage = restart()
	confirmChunkStorage(storage)
}