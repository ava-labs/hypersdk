package storage

import (
	"context"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/state"
)

type Recorder struct {
	State      state.Immutable
	ReadState  set.Set[string]
	WriteState set.Set[string]
}

func (r *Recorder) Insert(_ context.Context, key []byte, _ []byte) error {
	r.WriteState.Add(string(key))
	return nil
}

func (r *Recorder) Remove(_ context.Context, key []byte) error {
	r.WriteState.Add(string(key))
	return nil
}

func (r *Recorder) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	r.ReadState.Add(string(key))
	return r.State.GetValue(ctx, key)
}
