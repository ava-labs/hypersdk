package storage

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

const (
	// metaDB
	txPrefix = 0x0

	// stateDB
	keyPrefix = 0x0

	programPrefix      = 0x1
	heightPrefix       = 0x2
	timestampPrefix    = 0x3
	feePrefix          = 0x4
	incomingWarpPrefix = 0x5
	outgoingWarpPrefix = 0x6
)

func SetKey(ctx context.Context, db state.Mutable, privateKey ed25519.PrivateKey, name string) error {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], []byte(name))
	return db.Insert(ctx, k, privateKey[:])
}

// gets the public key mapped to the given name.
func GetPublicKey(ctx context.Context, db state.Immutable, name string) (ed25519.PublicKey, bool, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], []byte(name))
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPublicKey, false, nil
	}
	if err != nil {
		return ed25519.EmptyPublicKey, false, err
	}
	return ed25519.PublicKey(v), true, nil
}

func ProgramKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = programPrefix
	copy(k[1:], id[:])
	return
}

// setProgram stores [program] at [programID]
func SetProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	k := ProgramKey(programID)
	return mu.Insert(ctx, k, program)
}

// [programID] -> [programBytes]
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	error,
) {
	k := ProgramKey(programID)
	return db.GetValue(ctx, k)
}
