package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

type Manager interface {
	GetFaucetAddress(context.Context) (ed25519.PublicKey, error)
	GetChallenge(context.Context) ([]byte, uint16, error)
	SolveChallenge(context.Context, ed25519.PublicKey, []byte, []byte) (ids.ID, error)
}
