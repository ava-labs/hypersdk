package storage

import (
	"context"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/state"
)

type Recorder struct {
	State         database.Database
	changedValues map[string][]byte
	ReadState     set.Set[string]
	WriteState    set.Set[string]
}

func (r *Recorder) Insert(_ context.Context, key []byte, value []byte) error {
	stringKey := string(key)
	r.WriteState.Add(stringKey)
	r.changedValues[stringKey] = value
	return nil
}

func (r *Recorder) Remove(_ context.Context, key []byte) error {
	stringKey := string(key)
	r.WriteState.Add(stringKey)
	r.changedValues[stringKey] = nil
	return nil
}

func (r *Recorder) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	stringKey := string(key)
	r.ReadState.Add(stringKey)
	if value, ok := r.changedValues[stringKey]; ok {
		if value == nil {
			return nil, database.ErrNotFound
		}
		return value, nil
	}
	return r.State.Get(key)
}

func (r *Recorder) GetStateKeys() state.Keys {
	result := state.Keys{}
	for key := range r.ReadState {
		result.Add(key, state.Read)
	}
	for key := range r.WriteState {
		result.Add(key, state.Write|state.Allocate)
	}
	return result
}
