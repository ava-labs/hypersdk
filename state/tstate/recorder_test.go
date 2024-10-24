package tstate

import (
	"crypto/rand"
	"testing"

	"github.com/ava-labs/hypersdk/keys"
)

func FuzzRecorderPermissionValidator(f *testing.F) {
	for _, opCount := range []int{1, 3, 32, 128} {
		f.Add(opCount)
	}
	f.Fuzz(
		RecorderPermissionValidatorFuzzer,
	)
}

func createKeys() (keys1 [][]byte, keys2 [][]byte) {
	for i := 0; i < 1024; i++ {
		randNewKey := make([]byte, 30, 32)
		_, err := rand.Read(randNewKey)
		if err != nil {
			panic(err)
		}
		keys1 = append(keys1, keys.EncodeChunks(randNewKey, 1))
	}
	keys2 = keys1[len(keys1)/2:]
	keys1 = keys1[:len(keys1)/2]
	return
}

func createKeysValues(keys [][]byte) (out map[string][]byte) {
	out = map[string][]byte{}
	for i := range keys {
		randNewValue := make([]byte, 32, 32)
		_, err := rand.Read(randNewValue)
		if err != nil {
			panic(err)
		}
		out[string(keys[i])] = randNewValue
	}
	return out
}

func RecorderPermissionValidatorFuzzer(t *testing.T, opCount int) {
	// create a set of keys which would be used for testing.
	// half of these keys would "exists", where the other won't.
	existingKeys, nonExistingKeys := createKeys()
	existingKeyValue := createKeysValues(existingKeys)
}
