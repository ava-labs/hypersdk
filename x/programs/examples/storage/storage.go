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

//
// Program
//

func ProgramKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	// convert uint64 to bytes
	k[0] = programPrefix
	copy(k[1:], id[:])
	return
}

// [programID] -> [programBytes]
func GetProgram(
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := ProgramKey(programID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
	db database.KeyValueWriter,
	programID ids.ID,
	program []byte,
) error {
	k := ProgramKey(programID)
	v := make([]byte, len(program))
	copy(v, program)
	return db.Put(k, v)
}

//
// Balance
//

// [accountPrefix] + [address] + [asset]
func BalanceKey(pk ed25519.PublicKey, asset ids.ID) (k []byte) {
	k = balanceKeyPool.Get().([]byte)
	k[0] = balancePrefix
	copy(k[1:], pk[:])
	copy(k[1+ed25519.PublicKeyLen:], asset[:])
	binary.BigEndian.PutUint16(k[1+ed25519.PublicKeyLen+consts.IDLen:], BalanceChunks)
	return
}

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	pk ed25519.PublicKey,
	asset ids.ID,
) (uint64, error) {
	key, bal, _, err := getBalance(ctx, im, pk, asset)
	balanceKeyPool.Put(key)
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	pk ed25519.PublicKey,
	asset ids.ID,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(pk, asset)
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
	asset ids.ID,
	balance uint64,
) error {
	k := BalanceKey(pk, asset)
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

func DeleteBalance(
	ctx context.Context,
	mu state.Mutable,
	pk ed25519.PublicKey,
	asset ids.ID,
) error {
	return mu.Remove(ctx, BalanceKey(pk, asset))
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	pk ed25519.PublicKey,
	asset ids.ID,
	amount uint64,
	create bool,
) error {
	key, bal, exists, err := getBalance(ctx, mu, pk, asset)
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
			"%w: could not add balance (asset=%s, bal=%d, addr=%s, amount=%d)",
			ErrInvalidBalance,
			asset,
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
	asset ids.ID,
	amount uint64,
) error {
	key, bal, _, err := getBalance(ctx, mu, pk, asset)
	if err != nil {
		return err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract balance (asset=%s, bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			asset,
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
