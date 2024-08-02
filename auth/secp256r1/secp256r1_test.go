// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256r1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
)

func TestSecp256r1Sign(t *testing.T) {
	require := require.New(t)

	priv, err := secp256r1.GeneratePrivateKey()
	require.NoError(err)

	factory := NewSECP256R1Factory(priv)

	message := []byte("Hello, world!")
	bls, err := factory.Sign(message)
	require.NoError(err)

	ctx := context.Background()
	err = bls.Verify(ctx, message)
	require.NoError(err)

	wrongMessage := []byte("Avalanche")
	require.NotEqual(message, wrongMessage)
	err = bls.Verify(ctx, wrongMessage)
	require.ErrorIs(err, crypto.ErrInvalidSignature)
}
