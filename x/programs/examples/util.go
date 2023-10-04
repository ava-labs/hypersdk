// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func newKeyPtr(ctx context.Context, key ed25519.PublicKey, runtime runtime.Runtime) (uint64, error) {
	ptr, err := runtime.Memory().Alloc(ed25519.PublicKeyLen)
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	err = runtime.Memory().Write(ptr, key[:])
	if err != nil {
		return 0, err
	}

	return ptr, err
}

func newKey() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPrivateKey, ed25519.EmptyPublicKey, err
	}

	return priv, priv.PublicKey(), nil
}
