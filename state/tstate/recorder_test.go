// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tstate

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/keys"
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
		shaBytes := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
		bytes := shaBytes[:]

		f.Add(bytes[0], // opCount
			binary.BigEndian.Uint64(bytes[1:9]), // keyPrefix
			bytes[10:])                          // randBytes
	}
	f.Fuzz(
		RecorderPermissionValidatorFuzzer,
	)
}

func createKeys(prefix uint64) [][]byte {
	var keylist [][]byte
	bytesPrefix := binary.AppendUvarint([]byte{}, prefix)
	for i := 0; i < 512; i++ {
		shaBytes := sha256.Sum256(append(bytesPrefix, []byte{byte(i), byte(i >> 8)}...))
		randNewKey := make([]byte, 30, 32)
		copy(randNewKey, shaBytes[:])
		keylist = append(keylist, keys.EncodeChunks(randNewKey, 1))
	}
	return keylist
}

func createKeysValues(keys [][]byte) map[string][]byte {
	out := map[string][]byte{}
	for i, key := range keys {
		shaBytes := sha256.Sum256(key)
		out[string(keys[i])] = shaBytes[:]
	}
	return out
}

func (i immutableScopeStorage) duplicate() immutableScopeStorage {
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

func RecorderPermissionValidatorFuzzer(t *testing.T, operationCount byte, keysPrefix uint64, randBytes []byte) {
	require := require.New(t)
	// create a set of keys which would be used for testing.
	// half of these keys would "exists", where the other won't.
	existingKeys := createKeys(keysPrefix)
	nonExistingKeys := createKeys(keysPrefix + 1)
	existingKeyValue := createKeysValues(existingKeys)

	// create a long living recorder.
	recorder := NewRecorder(immutableScopeStorage(existingKeyValue).duplicate())

	for opIdx := byte(0); opIdx < operationCount; opIdx++ {
		updatedSeed := sha256.Sum256(append(binary.AppendUvarint([]byte{}, uint64(opIdx)), randBytes...))
		randBytes = updatedSeed[:]

		opType := binary.BigEndian.Uint64(randBytes) & fuzzerOpCount
		keyIdx := binary.BigEndian.Uint16(randBytes[8:])
		switch opType {
		case fuzzerOpInsertExistingKey: // insert existing key
			keyIdx %= uint16(len(existingKeys))
			key := existingKeys[keyIdx]
			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3}))

			existingKeyValue[string(key)] = []byte{1, 2, 3}
		case fuzzerOpInsertNonExistingKey: // insert non existing key
			keyIdx %= uint16(len(nonExistingKeys))
			key := nonExistingKeys[keyIdx]
			nonExistingKeys = removeSliceElement(nonExistingKeys, int(keyIdx))

			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3, 4}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3, 4}))

			// since we've modified the recorder state, we need to update our own.
			existingKeys = append(existingKeys, key)
			existingKeyValue[string(key)] = []byte{1, 2, 3, 4}
		case fuzzerOpRemoveExistingKey: // remove existing key
			keyIdx %= uint16(len(existingKeys))
			require.NoError(recorder.Remove(context.Background(), existingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), existingKeys[keyIdx]))

			// since we've modified the recorder state, we need to update our own.
			delete(existingKeyValue, string(existingKeys[keyIdx]))
			existingKeys = append(existingKeys[:keyIdx], existingKeys[keyIdx+1:]...)
		case fuzzerOpRemoveNonExistingKey: // remove a non existing key
			keyIdx %= uint16(len(nonExistingKeys))
			require.NoError(recorder.Remove(context.Background(), nonExistingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), nonExistingKeys[keyIdx]))
		case fuzzerOpGetExistingKey: // get value of existing key
			keyIdx %= uint16(len(existingKeys))
			recorderValue, err := recorder.GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			stateValue, err := New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue)).GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// both the recorder and the stateview should return the same value.
			require.Equal(recorderValue, stateValue)
		case fuzzerOpGetNonExistingKey: // get value of non existing key
			keyIdx %= uint16(len(nonExistingKeys))
			val, err := recorder.GetValue(context.Background(), nonExistingKeys[keyIdx])
			require.ErrorIs(err, database.ErrNotFound, "element was found with a value of %v, while it was supposed to be missing", val)

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			_, err = New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue)).GetValue(context.Background(), nonExistingKeys[keyIdx])
			require.ErrorIs(err, database.ErrNotFound)
		}
	}
}
