// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/bls"
)

func TestBLSSign(t *testing.T) {
	require := require.New(t)

	priv, err := bls.GeneratePrivateKey()
	require.NoError(err)

	factory := NewBLSFactory(priv)

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
