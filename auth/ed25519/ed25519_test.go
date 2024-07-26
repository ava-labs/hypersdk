// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	ced25519 "crypto/ed25519"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

var seed = []byte{
	0x00, 0x01, 0x02, 0x03, 0x04,
	0x05, 0x06, 0x07, 0x08, 0x09,
	0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
	0x0f, 0x10, 0x11, 0x12, 0x13,
	0x14, 0x15, 0x16, 0x17, 0x18,
	0x19, 0x1a, 0x1b, 0x1c, 0x1d,
	0x1e, 0x1f,
}

func TestEd25519SignVerify(t *testing.T) {
	priv := ed25519.PrivateKey(ced25519.NewKeyFromSeed(seed))

	factory := NewED25519Factory(priv)

	tests := []struct {
		name              string
		message           []byte
		signature         []byte
		expectedSignature []byte
		err               error
	}{
		{
			name:    "hello world",
			message: []byte("hello world"),
			expectedSignature: []byte{
				0xc9, 0xe8, 0x8a, 0x6, 0xc8,
				0x88, 0x55, 0xaa, 0x75, 0xf9,
				0xb, 0xcf, 0xdc, 0x5a, 0x87,
				0xb7, 0x6a, 0x99, 0xc0, 0xd2,
				0x4, 0x41, 0x14, 0xb8, 0x93,
				0x1e, 0x72, 0x8, 0x9e, 0x7b,
				0x8c, 0x7a, 0xc6, 0xb4, 0xa9,
				0x77, 0x6b, 0x57, 0x32, 0x6f,
				0x2d, 0x78, 0x1a, 0xa8, 0xda,
				0x88, 0x21, 0xfe, 0x6b, 0x4c,
				0x72, 0x96, 0xfd, 0xe0, 0xb6,
				0x3c, 0xa2, 0x4d, 0x7f, 0x63,
				0x43, 0xac, 0x6a, 0xa,
			},
		},
		{
			name:    "valid sig long string",
			message: []byte("a really long string"),
			expectedSignature: []byte{
				0xf6, 0x24, 0xb4, 0x5d, 0x1f,
				0xdd, 0x61, 0x7e, 0xdf, 0x31,
				0x16, 0xdf, 0xb5, 0x4c, 0x73,
				0x20, 0x29, 0xa0, 0xea, 0xd8,
				0xb4, 0x64, 0xe9, 0xab, 0x24,
				0x6c, 0x16, 0x88, 0xb3, 0xf5,
				0x51, 0x3f, 0xc2, 0x51, 0x22,
				0xd4, 0xcb, 0x89, 0xca, 0xbc,
				0x36, 0xbd, 0x68, 0xe2, 0xf6,
				0x67, 0xb8, 0x22, 0x27, 0x1,
				0x27, 0xf3, 0xf, 0xa6, 0xa5,
				0xa0, 0xe1, 0x5, 0x83, 0x91,
				0xed, 0x12, 0xe4, 0x4,
			},
		},
		{
			name:    "wrong signature",
			message: []byte("hello world"),
			// sig of "hello, world"
			signature: []byte{
				0x56, 0x73, 0xfe, 0xd6, 0x7b,
				0x41, 0x2e, 0xd0, 0x3, 0x4b,
				0xb1, 0xce, 0x1a, 0xba, 0x9c,
				0xf7, 0xcc, 0x56, 0x20, 0xa2,
				0xa4, 0x3f, 0xb, 0x2, 0xe9,
				0x6a, 0x56, 0x8c, 0xf3, 0x7e,
				0x30, 0x45, 0x22, 0xbd, 0x0,
				0x5a, 0x16, 0x5b, 0xbb, 0x3f,
				0x83, 0x5c, 0x60, 0x5e, 0x88,
				0xc8, 0xe8, 0xb1, 0x6, 0x52,
				0x5, 0x87, 0xd4, 0x46, 0x3f,
				0xc0, 0x3, 0x81, 0xdc, 0x26,
				0x43, 0x76, 0x4d, 0x2,
			},
			err: crypto.ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			auth, err := factory.Sign(tt.message)
			require.NoError(err)
			b := auth.(*ED25519)

			if tt.expectedSignature != nil {
				require.Equal(ed25519.Signature(tt.expectedSignature), b.Signature)
			}

			if tt.signature != nil {
				b.Signature = ed25519.Signature(tt.signature)
			}

			ctx := context.Background()
			err = auth.Verify(ctx, tt.message)
			require.Equal(tt.err, err)
		})
	}
}

func TestEd25519BatchVerify(t *testing.T) {
	require := require.New(t)

	priv := ed25519.PrivateKey(ced25519.NewKeyFromSeed(seed))

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

func TestEd25519BatchVerifyFails(t *testing.T) {
	require := require.New(t)

	priv := ed25519.PrivateKey(ced25519.NewKeyFromSeed(seed))

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
			Msg:   []byte("Hello, world 1!"),
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
			require.Equal(err, crypto.ErrInvalidSignature)
		} else {
			require.Nil(verifyFunc)
		}
	}
}
