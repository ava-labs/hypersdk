// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"
)

var errInvalidTestItem = errors.New("invalid test item")

var _ Verifier[Tx] = testVerifier[Tx]{}

var testDefaultProducer = ids.GenerateTestNodeID()

type testVerifier[T Tx] struct {
	correctIDs   set.Set[ids.ID]
	correctCerts set.Set[*ChunkCertificate]
}

func (testVerifier[T]) SetMin(int64) {}

func (t testVerifier[T]) Verify(chunk Chunk[T]) error {
	if t.correctIDs.Contains(chunk.id) {
		return nil
	}
	return fmt.Errorf("%w: %s", errInvalidTestItem, chunk.id)
}

func (t testVerifier[T]) VerifyCertificate(_ context.Context, cert *ChunkCertificate) error {
	if t.correctCerts.Contains(cert) {
		return nil
	}
	return fmt.Errorf("%w: %s", errInvalidTestItem, cert.ChunkID)
}

func createTestStorage(t *testing.T, validChunkExpiry, invalidChunkExpiry []int64, ruleFactory RuleFactory) (
	*ChunkStorage[dsmrtest.Tx],
	[]Chunk[dsmrtest.Tx],
	[]Chunk[dsmrtest.Tx],
	func() *ChunkStorage[dsmrtest.Tx],
	testVerifier[dsmrtest.Tx],
) {
	require := require.New(t)

	validChunks := make([]Chunk[dsmrtest.Tx], 0, len(validChunkExpiry))
	for _, expiry := range validChunkExpiry {
		chunk, err := newChunk(
			UnsignedChunk[dsmrtest.Tx]{
				Producer:    testDefaultProducer,
				Beneficiary: codec.Address{},
				Expiry:      expiry,
				Txs:         []dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
			},
			[48]byte{},
			[96]byte{},
		)
		require.NoError(err)
		validChunks = append(validChunks, chunk)
	}

	invalidChunks := make([]Chunk[dsmrtest.Tx], 0, len(invalidChunkExpiry))
	for _, expiry := range invalidChunkExpiry {
		chunk, err := newChunk(
			UnsignedChunk[dsmrtest.Tx]{
				Producer:    testDefaultProducer,
				Beneficiary: codec.Address{},
				Expiry:      expiry,
				Txs:         []dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
			},
			[48]byte{},
			[96]byte{},
		)
		require.NoError(err)
		invalidChunks = append(invalidChunks, chunk)
	}

	tempDir := t.TempDir()

	db, err := pebble.New(tempDir, pebble.NewDefaultConfig(), prometheus.NewRegistry())
	require.NoError(err)
	validChunkIDs := make([]ids.ID, 0, len(validChunks))
	for _, chunk := range validChunks {
		validChunkIDs = append(validChunkIDs, chunk.id)
	}

	testVerifier := testVerifier[dsmrtest.Tx]{correctIDs: set.Of(validChunkIDs...), correctCerts: set.NewSet[*ChunkCertificate](0)}
	storage, err := NewChunkStorage[dsmrtest.Tx](
		testVerifier,
		db,
		ruleFactory,
	)
	require.NoError(err)

	restart := func() *ChunkStorage[dsmrtest.Tx] {
		require.NoError(db.Close())
		db, err = pebble.New(tempDir, pebble.NewDefaultConfig(), prometheus.NewRegistry())
		require.NoError(err)

		storage, err := NewChunkStorage[dsmrtest.Tx](
			testVerifier,
			db,
			ruleFactory,
		)
		require.NoError(err)
		return storage
	}
	return storage, validChunks, invalidChunks, restart, testVerifier
}

func TestStoreAndSaveValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _, verifier := createTestStorage(t, []int64{time.Now().Unix()}, []int64{}, testRuleFactory)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.NoError(err)

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
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
		Signature: &warp.BitSetSignature{},
	}
	// test to see that the storage cert verification fails.
	require.ErrorIs(storage.SetChunkCert(context.Background(), chunk.id, chunkCert), errInvalidTestItem)
	// test to see that the storage cert verification passes.
	verifier.correctCerts.Add(chunkCert)
	require.NoError(storage.SetChunkCert(context.Background(), chunk.id, chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, []ids.ID{chunk.id}))

	foundAcceptedChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireValidChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _, verifier := createTestStorage(t, []int64{time.Now().Unix()}, []int64{}, testRuleFactory)
	chunk := validChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.NoError(err)

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
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
		Signature: &warp.BitSetSignature{},
	}
	verifier.correctCerts.Add(chunkCert)
	require.NoError(storage.SetChunkCert(context.Background(), chunk.id, chunkCert))
	chunkCerts = storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, nil))

	_, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.ErrorIs(err, database.ErrNotFound)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreInvalidChunk(t *testing.T) {
	require := require.New(t)

	storage, _, invalidChunks, _, _ := createTestStorage(t, []int64{}, []int64{time.Now().Unix()}, testRuleFactory)
	chunk := invalidChunks[0]

	_, err := storage.VerifyRemoteChunk(chunk)
	require.ErrorIs(err, errInvalidTestItem)

	_, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.ErrorIs(err, database.ErrNotFound)

	chunkCerts := storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndSaveLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _, _ := createTestStorage(t, []int64{time.Now().Unix()}, []int64{}, testRuleFactory)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
		Signature: &warp.BitSetSignature{},
	}

	require.NoError(storage.AddLocalChunkWithCert(chunk, chunkCert))

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, []ids.ID{chunk.id}))

	foundAcceptedChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundAcceptedChunkBytes)
	chunkCerts = storage.GatherChunkCerts()
	require.Empty(chunkCerts)
}

func TestStoreAndExpireLocalChunk(t *testing.T) {
	require := require.New(t)

	storage, validChunks, _, _, _ := createTestStorage(t, []int64{time.Now().Unix()}, []int64{}, testRuleFactory)
	chunk := validChunks[0]
	chunkCert := &ChunkCertificate{
		ChunkReference: ChunkReference{
			ChunkID:  ids.GenerateTestID(),
			Producer: ids.GenerateTestNodeID(),
			Expiry:   1,
		},
		Signature: &warp.BitSetSignature{},
	}

	require.NoError(storage.AddLocalChunkWithCert(chunk, chunkCert))

	foundChunkBytes, err := storage.GetChunkBytes(chunk.Expiry, chunk.id)
	require.NoError(err)
	require.Equal(chunk.bytes, foundChunkBytes)

	chunkCerts := storage.GatherChunkCerts()
	require.Len(chunkCerts, 1)
	require.Equal(chunkCert, chunkCerts[0])

	require.NoError(storage.SetMin(chunk.Expiry+1, nil))

	_, err = storage.GetChunkBytes(chunk.Expiry, chunk.id)
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
	storage, validChunks, _, restart, verifier := createTestStorage(t, []int64{2, 2, 1, 1, 2, 2}, []int64{}, testRuleFactory)
	chunkCerts := make([]*ChunkCertificate, 0, numChunks)
	for _, chunk := range validChunks {
		chunkCert := &ChunkCertificate{
			ChunkReference: ChunkReference{
				ChunkID:  chunk.id,
				Producer: chunk.Producer,
				Expiry:   chunk.Expiry,
			},
			Signature: &warp.BitSetSignature{},
		}
		chunkCerts = append(chunkCerts, chunkCert)
	}
	verifier.correctCerts.Add(chunkCerts[1])
	verifier.correctCerts.Add(chunkCerts[5])

	// Case 1
	require.NoError(storage.AddLocalChunkWithCert(validChunks[0], chunkCerts[0]))

	// Case 2
	_, err := storage.VerifyRemoteChunk(validChunks[1])
	require.NoError(err)
	require.NoError(storage.SetChunkCert(context.Background(), validChunks[1].id, chunkCerts[1]))

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
	require.NoError(storage.SetChunkCert(context.Background(), validChunks[5].id, chunkCerts[5]))

	// Set the minimum to 2 and mark cases 1 and 2 as saved
	require.NoError(storage.SetMin(2, []ids.ID{
		validChunks[0].id,
		validChunks[1].id,
	}))

	// Case 7
	// Call AddLocalChunkWithCert on a previously accepted chunk and make sure it remains accepted.
	err = storage.AddLocalChunkWithCert(validChunks[1], nil)
	require.NoError(err)
	_, err = storage.GetChunkBytes(validChunks[1].Expiry, validChunks[1].id)
	require.NoError(err)

	confirmChunkStorage := func(storage *ChunkStorage[dsmrtest.Tx]) {
		// Confirm we can fetch the chunk bytes for the accepted and pending chunks
		for i, expectedChunk := range []Chunk[dsmrtest.Tx]{
			validChunks[0],
			validChunks[1],
			validChunks[4],
			validChunks[5],
		} {
			foundChunkBytes, err := storage.GetChunkBytes(expectedChunk.Expiry, expectedChunk.id)
			require.NoError(err, i)
			require.Equal(expectedChunk.bytes, foundChunkBytes, i)
		}

		// Confirm the expired chunks are garbage collected
		for _, expectedChunk := range []Chunk[dsmrtest.Tx]{
			validChunks[2],
			validChunks[3],
		} {
			_, err = storage.GetChunkBytes(expectedChunk.Expiry, expectedChunk.id)
			require.ErrorIs(err, database.ErrNotFound)
		}
	}
	confirmChunkStorage(storage)
	storage = restart()
	confirmChunkStorage(storage)
}

func TestChunkProducerRateLimiting(t *testing.T) {
	chunk, err := newChunk(
		UnsignedChunk[dsmrtest.Tx]{
			Producer:    testDefaultProducer,
			Beneficiary: codec.Address{},
			Expiry:      1,
			Txs:         []dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
		},
		[48]byte{},
		[96]byte{},
	)
	require.NoError(t, err)
	chunkSize := uint64(len(chunk.bytes))

	testCases := []struct {
		name                 string
		expiryTimes          []int64
		newChunkExpiry       int64
		weightLimit          uint64
		minTime              int64
		acceptedChunksExpiry []int64
		expectedErr          error
	}{
		{
			name:           "success - first",
			expiryTimes:    []int64{},
			newChunkExpiry: 50,
			weightLimit:    chunkSize * 5,
			expectedErr:    nil,
		},
		{
			name:           "success - after",
			expiryTimes:    []int64{1},
			newChunkExpiry: 50,
			weightLimit:    chunkSize * 5,
			expectedErr:    nil,
		},
		{
			name:           "success - before",
			expiryTimes:    []int64{50},
			newChunkExpiry: 1,
			weightLimit:    chunkSize * 5,
			expectedErr:    nil,
		},
		{
			name:           "fail - localized saturated range",
			expiryTimes:    []int64{0, 50, 99, 150, 500},
			newChunkExpiry: 75,
			weightLimit:    chunkSize * 2,
			expectedErr:    ErrChunkRateLimitSurpassed,
		},
		{
			name:           "fail - localized saturated range with multiple elements",
			expiryTimes:    []int64{0, 100, 120, 150, 200},
			newChunkExpiry: 130,
			weightLimit:    chunkSize * 3,
			expectedErr:    ErrChunkRateLimitSurpassed,
		},
		{
			name:                 "success - accepted block clear previous limit",
			expiryTimes:          []int64{0, 50, 100},
			newChunkExpiry:       150,
			weightLimit:          chunkSize * 3,
			expectedErr:          nil,
			minTime:              10,
			acceptedChunksExpiry: []int64{50},
		},
		{
			name:                 "success - expired chunks clear previous limit",
			expiryTimes:          []int64{50, 100, 150},
			newChunkExpiry:       200,
			weightLimit:          chunkSize * 3,
			expectedErr:          nil,
			minTime:              55,
			acceptedChunksExpiry: []int64{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require := require.New(t)
			rules := &ruleFactory{
				rules: rules{
					maxProducerChunkWeight: testCase.weightLimit,
				},
			}
			storage, chunks, _, _, _ := createTestStorage(t, testCase.expiryTimes, []int64{}, rules)

			for _, chunk := range chunks {
				require.NoError(storage.AddLocalChunkWithCert(chunk, nil))
			}

			var acceptedChunks []ids.ID
			for _, chunkExpiry := range testCase.acceptedChunksExpiry {
				// find the chunk that corresponds to this expiry in the chunks slice.
				chunkIndex := slices.IndexFunc(chunks, func(chunk Chunk[dsmrtest.Tx]) bool {
					return chunk.Expiry == chunkExpiry
				})
				require.NotEqual(-1, chunkIndex, "acceptedChunksExpiry contains an expiry time missing from expiryTimes")
				acceptedChunks = append(acceptedChunks, chunks[chunkIndex].id)
			}
			require.NoError(storage.SetMin(testCase.minTime, acceptedChunks))

			chunk, err := newChunk(
				UnsignedChunk[dsmrtest.Tx]{
					Producer:    testDefaultProducer,
					Beneficiary: codec.Address{},
					Expiry:      testCase.newChunkExpiry,
					Txs:         []dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1_000_000}},
				},
				[48]byte{},
				[96]byte{},
			)
			require.NoError(err)

			require.ErrorIs(storage.CheckRateLimit(chunk), testCase.expectedErr)
		})
	}
}
