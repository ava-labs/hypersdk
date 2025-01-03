// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tstate

import (
	"context"
	"crypto/sha256"
	"math/rand"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"
)

const (
	fuzzerOpInsertExistingKey = iota
	fuzzerOpInsertNonExistingKey
	fuzzerOpRemoveExistingKey
	fuzzerOpRemoveNonExistingKey
	fuzzerOpGetExistingKey
	fuzzerOpGetNonExistingKey
	fuzzerOpCount
)

func FuzzRecorderPermissionValidator(f *testing.F) {
	for i := 0; i < 100; i++ {
		f.Add(byte(i), int64(i))
	}

	f.Fuzz(
		RecorderPermissionValidatorFuzzer,
	)
}

func createKeys(numKeys int, keySize int, rand *rand.Rand) [][]byte {
	keys := make([][]byte, numKeys)
	for i := 0; i < numKeys; i++ {
		randKey := make([]byte, keySize)
		_, err := rand.Read(randKey)
		if err != nil {
			panic(err)
		}
		keys[i] = randKey
	}
	return keys
}

func createKeysValues(keys [][]byte) map[string][]byte {
	out := map[string][]byte{}
	for i, key := range keys {
		shaBytes := sha256.Sum256(key)
		out[string(keys[i])] = shaBytes[:]
	}
	return out
}

func (i ImmutableScopeStorage) duplicate() ImmutableScopeStorage {
	other := make(map[string][]byte, len(i))
	for k, v := range i {
		other[k] = v
	}
	return other
}

// removeSliceElement removes the idx-th element from a slice without maintaining the original
// slice order. removeSliceElement modifies the backing array of the provided slice.
func removeSliceElement[K any](slice []K, idx int) []K {
	if idx >= len(slice) {
		return slice
	}
	// swap the element at index [idx] with the last element.
	slice[idx], slice[len(slice)-1] = slice[len(slice)-1], slice[idx]
	// resize the slice and return it.
	return slice[:len(slice)-1]
}

func RecorderPermissionValidatorFuzzer(t *testing.T, operationCount byte, source int64) {
	require := require.New(t)
	r := rand.New(rand.NewSource(source)) //nolint:gosec
	// create a set of keys which would be used for testing.
	// half of these keys would "exists", where the other won't.
	existingKeys := createKeys(512, 32, r)
	nonExistingKeys := createKeys(512, 32, r)
	existingKeyValue := createKeysValues(existingKeys)

	// create a long living recorder.
	recorder := NewRecorder(ImmutableScopeStorage(existingKeyValue).duplicate())

	for opIdx := byte(0); opIdx < operationCount; opIdx++ {
		opType := r.Intn(fuzzerOpCount)
		switch opType {
		case fuzzerOpInsertExistingKey: // insert existing key
			key := existingKeys[r.Intn(len(existingKeys))]
			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3}))

			existingKeyValue[string(key)] = []byte{1, 2, 3}
		case fuzzerOpInsertNonExistingKey: // insert non existing key
			keyIdx := r.Intn(len(nonExistingKeys))
			key := nonExistingKeys[keyIdx]
			nonExistingKeys = removeSliceElement(nonExistingKeys, keyIdx)

			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3, 4}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3, 4}))

			// since we've modified the recorder state, we need to update our own.
			existingKeys = append(existingKeys, key)
			existingKeyValue[string(key)] = []byte{1, 2, 3, 4}
		case fuzzerOpRemoveExistingKey: // remove existing key
			keyIdx := r.Intn(len(existingKeys))
			require.NoError(recorder.Remove(context.Background(), existingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), existingKeys[keyIdx]))

			// since we've modified the recorder state, we need to update our own.
			delete(existingKeyValue, string(existingKeys[keyIdx]))
			existingKeys = append(existingKeys[:keyIdx], existingKeys[keyIdx+1:]...)
		case fuzzerOpRemoveNonExistingKey: // remove a non existing key
			keyIdx := r.Intn(len(nonExistingKeys))
			require.NoError(recorder.Remove(context.Background(), nonExistingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), nonExistingKeys[keyIdx]))
		case fuzzerOpGetExistingKey: // get value of existing key
			keyIdx := r.Intn(len(existingKeys))
			recorderValue, err := recorder.GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			stateValue, err := New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue)).GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// both the recorder and the stateview should return the same value.
			require.Equal(recorderValue, stateValue)
		case fuzzerOpGetNonExistingKey: // get value of non existing key
			keyIdx := r.Intn(len(nonExistingKeys))
			val, err := recorder.GetValue(context.Background(), nonExistingKeys[keyIdx])
			require.ErrorIs(err, database.ErrNotFound, "element was found with a value of %v, while it was supposed to be missing", val)

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			_, err = New(0).NewView(stateKeys, ImmutableScopeStorage(existingKeyValue)).GetValue(context.Background(), nonExistingKeys[keyIdx])
			require.ErrorIs(err, database.ErrNotFound)
		}
	}
}
