// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func randomNewKey() []byte {
	randNewKey := make([]byte, 30, 32)
	_, err := rand.Read(randNewKey)
	if err != nil {
		panic(err)
	}
	return keys.EncodeChunks(randNewKey, 1)
}

func randomizeView(tstate *tstate.TState, keyCount int) (*tstate.TStateView, [][]byte, map[string]state.Permissions, map[string][]byte) {
	keys := make([][]byte, keyCount)
	values := make([][32]byte, keyCount)
	storage := map[string][]byte{}
	scope := map[string]state.Permissions{}
	for i := 0; i < keyCount; i++ {
		keys[i] = randomNewKey()
		_, err := rand.Read(values[i][:])
		if err != nil {
			panic(err)
		}
		storage[string(keys[i])] = values[i][:]
		scope[string(keys[i])] = state.All
	}
	// create new view
	return tstate.NewView(scope, storage), keys, scope, storage
}

func TestRecorderInnerFuzz(t *testing.T) {
	tstateObj := tstate.New(1000)
	require := require.New(t)

	var (
		stateView   *tstate.TStateView
		keys        [][]byte
		scope       map[string]state.Permissions
		removedKeys map[string]bool
	)

	pickExistingKeyAtRandom := func() []byte {
		randKey := make([]byte, 1)
		_, err := rand.Read(randKey)
		require.NoError(err)
		randKey[0] %= byte(len(keys))
		for removedKeys[string(keys[randKey[0]])] {
			_, err := rand.Read(randKey)
			randKey[0] %= byte(len(keys))
			require.NoError(err)
		}
		return keys[randKey[0]]
	}
	for i := 0; i < 10000; i++ {
		stateView, keys, scope, _ = randomizeView(tstateObj, 32)
		removedKeys = map[string]bool{}
		// wrap with recorder.
		recorder := state.NewRecorder(stateView)
		for j := 0; j <= 32; j++ {
			op := make([]byte, 1)
			_, err := rand.Read(op)
			require.NoError(err)
			switch op[0] % 6 {
			case 0: // insert into existing entry
				randKey := pickExistingKeyAtRandom()
				err := recorder.Insert(context.Background(), randKey, []byte{1, 2, 3, 4})
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))
			case 1: // insert into new entry
				randNewKey := randomNewKey()
				// add the new key to the scope
				scope[string(randNewKey)] = state.Allocate | state.Write
				err := recorder.Insert(context.Background(), randNewKey, []byte{1, 2, 3, 4})
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randNewKey)].Has(state.Allocate | state.Write))
				keys = append(keys, randNewKey)
			case 2: // remove existing entry
				randKey := pickExistingKeyAtRandom()
				err := recorder.Remove(context.Background(), randKey)
				require.NoError(err)
				removedKeys[string(randKey)] = true
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))
			case 3: // remove non existing entry
				randKey := randomNewKey()
				err := recorder.Remove(context.Background(), randKey)
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))
			case 4: // get value of existing entry
				randKey := pickExistingKeyAtRandom()
				val, err := recorder.GetValue(context.Background(), randKey)
				require.NoError(err)
				require.NotEmpty(val)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Read))
			case 5: // get value of non existing entry
				randKey := randomNewKey()
				// add the new key to the scope
				scope[string(randKey)] = state.Read
				value, err := recorder.GetValue(context.Background(), randKey)
				require.ErrorIs(err, database.ErrNotFound)
				require.Empty(value)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Read))
			}
		}
	}
}

type testingReadonlyDatasource struct {
	storage map[string][]byte
}

func (c *testingReadonlyDatasource) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	if v, has := c.storage[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func TestRecorderSideBySideFuzz(t *testing.T) {
	tstateObj := tstate.New(1000)
	require := require.New(t)

	var (
		stateView   *tstate.TStateView
		keys        [][]byte
		scope       map[string]state.Permissions
		removedKeys map[string]bool
		storage     map[string][]byte
	)

	pickExistingKeyAtRandom := func() []byte {
		randKey := make([]byte, 1)
		_, err := rand.Read(randKey)
		require.NoError(err)
		randKey[0] %= byte(len(keys))
		for removedKeys[string(keys[randKey[0]])] {
			_, err := rand.Read(randKey)
			randKey[0] %= byte(len(keys))
			require.NoError(err)
		}
		return keys[randKey[0]]
	}
	randomValue := func() []byte {
		randVal := make([]byte, 32)
		_, err := rand.Read(randVal)
		require.NoError(err)
		return randVal
	}

	for i := 0; i < 10000; i++ {
		stateView, keys, scope, storage = randomizeView(tstateObj, 32)
		removedKeys = map[string]bool{}
		// wrap with recorder.
		recorder := state.NewRecorder(&testingReadonlyDatasource{storage})
		for j := 0; j <= 32; j++ {
			op := make([]byte, 1)
			_, err := rand.Read(op)
			require.NoError(err)
			switch op[0] % 6 {
			case 0: // insert into existing entry
				randKey := pickExistingKeyAtRandom()
				randVal := randomValue()

				err := recorder.Insert(context.Background(), randKey, randVal)
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))

				err = stateView.Insert(context.Background(), randKey, randVal)
				require.NoError(err)
			case 1: // insert into new entry
				randNewKey := randomNewKey()
				randVal := randomValue()

				err := recorder.Insert(context.Background(), randNewKey, randVal)
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randNewKey)].Has(state.Allocate | state.Write))

				// add the new key to the scope
				scope[string(randNewKey)] = state.Write | state.Allocate
				err = stateView.Insert(context.Background(), randNewKey, randVal)
				require.NoError(err)

				keys = append(keys, randNewKey)
			case 2: // remove existing entry
				randKey := pickExistingKeyAtRandom()

				err := recorder.Remove(context.Background(), randKey)
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))

				err = stateView.Remove(context.Background(), randKey)
				require.NoError(err)

				removedKeys[string(randKey)] = true
			case 3: // remove non existing entry
				randKey := randomNewKey()

				err := recorder.Remove(context.Background(), randKey)
				require.NoError(err)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Write))

				// add the new key to the scope
				scope[string(randKey)] = state.Write
				err = stateView.Remove(context.Background(), randKey)
				require.NoError(err)
			case 4: // get value of existing entry
				randKey := pickExistingKeyAtRandom()

				val, err := recorder.GetValue(context.Background(), randKey)
				require.NoError(err)
				require.NotEmpty(val)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Read))

				val, err = stateView.GetValue(context.Background(), randKey)
				require.NoError(err)
				require.NotEmpty(val)
			case 5: // get value of non existing entry
				randKey := randomNewKey()

				value, err := recorder.GetValue(context.Background(), randKey)
				require.ErrorIs(err, database.ErrNotFound)
				require.Empty(value)
				require.True(recorder.GetStateKeys()[string(randKey)].Has(state.Read))

				// add the new key to the scope
				scope[string(randKey)] = state.Read
				value, err = stateView.GetValue(context.Background(), randKey)
				require.ErrorIs(err, database.ErrNotFound)
				require.Empty(value)
			}
		}
	}
}
