// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tstate

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/keys"
)

func FuzzRecorderPermissionValidator(f *testing.F) {
	for i := 0; i < 10; i++ {
		shaBytes := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
		bytes := shaBytes[:]
		for bytes[len(bytes)-1] > 16 {
			shaBytes := sha256.Sum256([]byte{bytes[len(bytes)-2], bytes[len(bytes)-1]})
			bytes = append(bytes, shaBytes[:]...)
		}
		f.Add(bytes)
	}
	f.Fuzz(
		RecorderPermissionValidatorFuzzer,
	)
}

func createKeys(prefix byte) (keylist [][]byte) {
	for i := 0; i < 512; i++ {
		shaBytes := sha256.Sum256([]byte{prefix, byte(i), byte(i >> 8)})
		randNewKey := make([]byte, 30, 32)
		copy(randNewKey, shaBytes[:])
		keylist = append(keylist, keys.EncodeChunks(randNewKey, 1))
	}
	return
}

func createKeysValues(keys [][]byte) (out map[string][]byte) {
	out = map[string][]byte{}
	for i, key := range keys {
		shaBytes := sha256.Sum256(key)
		out[string(keys[i])] = shaBytes[:]
	}
	return out
}

func nextByte(randBytes []byte) (byte, []byte, bool) {
	if len(randBytes) == 0 {
		return 0, randBytes, true
	}
	return randBytes[0], randBytes[1:], false
}

func nextUint16(randBytes []byte) (uint16, []byte, bool) {
	if len(randBytes) < 2 {
		return 0, randBytes, true
	}
	return uint16(randBytes[0]) | (uint16(randBytes[0]) << 8), randBytes[2:], false
}

func (i immutableScopeStorage) duplicate() immutableScopeStorage {
	other := make(map[string][]byte, len(i))
	for k, v := range i {
		other[k] = v
	}
	return other
}

func RecorderPermissionValidatorFuzzer(t *testing.T, randBytes []byte) {
	require := require.New(t)
	// create a set of keys which would be used for testing.
	// half of these keys would "exists", where the other won't.
	existingKeys := createKeys(0)
	nonExistingKeys := createKeys(1)
	existingKeyValue := createKeysValues(existingKeys)

	// create a long living recorder.
	recorder := NewRecorder(immutableScopeStorage(existingKeyValue).duplicate())

	operationCount, randBytes, done := nextByte(randBytes)
	if done {
		return
	}
	operationCount %= 128 // limit to 128 operations.
	var opType byte
	for opIdx := byte(0); opIdx < operationCount; opIdx++ {
		if opType, randBytes, done = nextByte(randBytes); done {
			return
		}
		opType %= 6
		var keyIdx uint16
		if keyIdx, randBytes, done = nextUint16(randBytes); done {
			return
		}
		switch opType {
		case 0: // insert existing key
			keyIdx %= uint16(len(existingKeys))
			key := existingKeys[keyIdx]
			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3}))

			existingKeyValue[string(key)] = []byte{1, 2, 3}
		case 1: // insert non existing key
			keyIdx %= uint16(len(nonExistingKeys))
			key := nonExistingKeys[keyIdx]
			nonExistingKeys[keyIdx] = []byte{}
			nonExistingKeys = append(nonExistingKeys[:keyIdx], nonExistingKeys[keyIdx+1:]...)

			require.NoError(recorder.Insert(context.Background(), key, []byte{1, 2, 3, 4}))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Insert(context.Background(), key, []byte{1, 2, 3}))

			// since we've modified the recorder state, we need to update our own.
			existingKeys = append(existingKeys, key)
			existingKeyValue[string(key)] = []byte{1, 2, 3, 4}
		case 2: // remove existing key
			keyIdx %= uint16(len(existingKeys))
			require.NoError(recorder.Remove(context.Background(), existingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), existingKeys[keyIdx]))

			// since we've modified the recorder state, we need to update our own.
			delete(existingKeyValue, string(existingKeys[keyIdx]))
			existingKeys = append(existingKeys[:keyIdx], existingKeys[keyIdx+1:]...)
		case 3: // remove a non existing key
			keyIdx %= uint16(len(nonExistingKeys))
			require.NoError(recorder.Remove(context.Background(), nonExistingKeys[keyIdx]))

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			require.NoError(New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue).duplicate()).Remove(context.Background(), nonExistingKeys[keyIdx]))
		case 4: // get value of existing key
			keyIdx %= uint16(len(existingKeys))
			recorderValue, err := recorder.GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// validate operation agaist TStateView
			stateKeys := recorder.GetStateKeys()
			stateValue, err := New(0).NewView(stateKeys, immutableScopeStorage(existingKeyValue)).GetValue(context.Background(), existingKeys[keyIdx])
			require.NoError(err)

			// both the recorder and the stateview should return the same value.
			require.Equal(recorderValue, stateValue)
		case 5: // get value of non existing key
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
