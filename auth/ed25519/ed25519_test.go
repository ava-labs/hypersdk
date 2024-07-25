// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

func TestEd25519Sign(t *testing.T) {
	require := require.New(t)

	priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	factory := NewED25519Factory(priv)

	message := []byte("Hello, world!")
	ed25519, err := factory.Sign(message)
	require.NoError(err)

	ctx := context.Background()
	err = ed25519.Verify(ctx, message)
	require.NoError(err)

	wrongMessage := []byte("Avalanche")
	require.NotEqual(message, wrongMessage)
	err = ed25519.Verify(ctx, wrongMessage)
	require.ErrorIs(err, crypto.ErrInvalidSignature)
}

func TestEd25519Batch(t *testing.T) {
	require := require.New(t)

	priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	factory := NewED25519Factory(priv)

	type SignedMessage struct {
		Msg   []byte
		Rauth chain.Auth
	}

	cores := 2
	count := 10
	batchSize := max(count/cores, ed25519.MinBatchSize)
	signedMessages := make([]SignedMessage, count)

	for i := 0; i < count; i++ {
		message := []byte(fmt.Sprintf("Hello, world %d!", i))
		ed25519, err := factory.Sign(message)
		require.NoError(err)
		signedMessages[i] = SignedMessage{
			Msg:   message,
			Rauth: ed25519,
		}
	}

	engine := &ED25519AuthEngine{}
	verifier := engine.GetBatchVerifier(cores, count)

	for i, signedMessage := range signedMessages {
		verifyFunc := verifier.Add(signedMessage.Msg, signedMessage.Rauth)
		if (i+1)%batchSize == 0 {
			require.NotNil(verifyFunc)
			err := verifyFunc()
			require.NoError(err)
		} else {
			require.Nil(verifyFunc)
		}
	}
}
