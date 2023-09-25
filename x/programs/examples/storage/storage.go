// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

const (
	HRP           = "simulator_key_"
	programPrefix = 0x0
	balancePrefix = 0x1

	BalanceChunks uint16 = 1
)

var (
	ErrInvalidBalance = errors.New("invalid balance")
	balanceKeyPool    = sync.Pool{
		New: func() any {
			return make([]byte, 1+ed25519.PublicKeyLen+consts.IDLen+consts.Uint16Len)
		},
	}
)

func ProgramPrefixKey(id []byte, key []byte) (k []byte) {
	k = make([]byte, consts.IDLen+1+len(key))
	k[0] = programPrefix
	copy(k, id[:])
	copy(k[consts.IDLen:], (key[:]))
	return
}

//
// Program
//

func ProgramKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	copy(k[1:], id[:])
	return
}

// [programID] -> [programBytes]
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := ProgramKey(programID)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// SetProgram stores [program] at [programID]
func SetProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	k := ProgramKey(programID)
	return mu.Insert(ctx, k, program)
}

//
// Balance
//

// [balancePrefix] + [address]
func BalanceKey(pk ed25519.PublicKey) (k []byte) {
	k = balanceKeyPool.Get().([]byte)
	k[0] = balancePrefix
	copy(k[1:], pk[:])
	binary.BigEndian.PutUint16(k[1+ed25519.PublicKeyLen:], BalanceChunks)
	return
}

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	pk ed25519.PublicKey,
) (uint64, error) {
	key, bal, _, err := getBalance(ctx, im, pk)
	balanceKeyPool.Put(key)
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	pk ed25519.PublicKey,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(pk)
	bal, exists, err := innerGetBalance(im.GetValue(ctx, k))
	return k, bal, exists, err
}

func innerGetBalance(
	v []byte,
	err error,
) (uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return binary.BigEndian.Uint64(v), true, nil
}

func SetBalance(
	ctx context.Context,
	mu state.Mutable,
	pk ed25519.PublicKey,
	balance uint64,
) error {
	k := BalanceKey(pk)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key []byte,
	balance uint64,
) error {
	return mu.Insert(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	pk ed25519.PublicKey,
	amount uint64,
	create bool,
) error {
	key, bal, exists, err := getBalance(ctx, mu, pk)
	if err != nil {
		return err
	}
	// Don't add balance if account doesn't exist. This
	// can be useful when processing fee refunds.
	if !exists && !create {
		return nil
	}
	nbal, err := smath.Add64(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			ed25519.Address(HRP, pk),
			amount,
		)
	}
	return setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	pk ed25519.PublicKey,
	amount uint64,
) error {
	key, bal, _, err := getBalance(ctx, mu, pk)
	if err != nil {
		return err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			ed25519.Address(HRP, pk),
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return mu.Remove(ctx, key)
	}
	return setBalance(ctx, mu, key, nbal)
}