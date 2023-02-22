// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"

	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// Metadata
// 0x0/ (tx)
//   -> [txID] => timestamp
//
// State
// 0x0/ (balance)
//   -> [owner] => unlockedBalance|lockedBalance
// 0x1/ (permissions)
//   -> [actor|signer] => permissions bitset
// 0x2/ (content)
//   -> [contentID] => owner|royalty

const (
	txPrefix = 0x0

	balancePrefix     = 0x0
	permissionsPrefix = 0x1
	contentPrefix     = 0x2
)

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)
)

// [txPrefix] + [txID]
func PrefixTxKey(id ids.ID) (k []byte) {
	// TODO: use packer?
	k = make([]byte, 1+consts.IDLen)
	k[0] = txPrefix
	copy(k[1:], id[:])
	return
}

func StoreTransaction(
	_ context.Context,
	db database.KeyValueWriter,
	id ids.ID,
	t int64,
	success bool,
	units uint64,
) error {
	k := PrefixTxKey(id)
	v := make([]byte, consts.Uint64Len+1+consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(t))
	if success {
		v[consts.Uint64Len] = successByte
	} else {
		v[consts.Uint64Len] = failureByte
	}
	binary.BigEndian.PutUint64(v[consts.Uint64Len+1:], units)
	return db.Put(k, v)
}

func GetTransaction(
	_ context.Context,
	db database.KeyValueReader,
	id ids.ID,
) (bool, int64, bool, uint64, error) {
	k := PrefixTxKey(id)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, 0, nil
	}
	if err != nil {
		return false, 0, false, 0, err
	}
	t := int64(binary.BigEndian.Uint64(v))
	success := true
	if v[consts.Uint64Len] == failureByte {
		success = false
	}
	units := binary.BigEndian.Uint64(v[consts.Uint64Len+1:])
	return true, t, success, units, nil
}

// [accountPrefix] + [address]
func PrefixBalanceKey(pk crypto.PublicKey) (k []byte) {
	k = make([]byte, 1+ed25519.PublicKeySize)
	k[0] = balancePrefix
	copy(k[1:], pk[:])
	return
}

// [contentPrefix] + [contentID]
func PrefixContentKey(contentID ids.ID) (k []byte) {
	k = make([]byte, 33)
	k[0] = contentPrefix
	copy(k[1:], contentID[:])
	return
}

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
) (uint64 /* unlocked */, uint64 /* locked */, error) {
	k := PrefixBalanceKey(pk)
	return innerGetBalance(db.GetValue(ctx, k))
}

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	pk crypto.PublicKey,
) (uint64 /* unlocked */, uint64 /* locked */, error) {
	values, errs := f(ctx, [][]byte{PrefixBalanceKey(pk)})
	return innerGetBalance(values[0], errs[0])
}

func innerGetBalance(
	v []byte,
	err error,
) (uint64 /* unlocked */, uint64 /* locked */, error) {
	if errors.Is(err, database.ErrNotFound) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return binary.BigEndian.Uint64(v[:8]), binary.BigEndian.Uint64(v[8:]), nil
}

func SetBalance(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
	unlockedBal uint64,
	lockedBal uint64,
) error {
	k := PrefixBalanceKey(pk)
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b, unlockedBal)
	binary.BigEndian.PutUint64(b[8:], lockedBal)
	return db.Insert(ctx, k, b)
}

func DeleteBalance(ctx context.Context, db chain.Database, pk crypto.PublicKey) error {
	return db.Remove(ctx, PrefixBalanceKey(pk))
}

// [dropIfGone] is used if the address is cleared out
func AddUnlockedBalance(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
	amount uint64,
	dropIfGone bool,
) (bool, error) {
	ubal, lbal, err := GetBalance(ctx, db, pk)
	if err != nil {
		return false, err
	}
	if dropIfGone && lbal == 0 {
		return false, nil
	}
	nbal, err := smath.Add64(ubal, amount)
	if err != nil {
		return false, fmt.Errorf(
			"%w: could not add unlocked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			ubal,
			utils.Address(pk),
			amount,
		)
	}
	return lbal > 0, SetBalance(ctx, db, pk, nbal, lbal)
}

func SubUnlockedBalance(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
	amount uint64,
) error {
	ubal, lbal, err := GetBalance(ctx, db, pk)
	if err != nil {
		return err
	}
	nbal, err := smath.Sub(ubal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract unlocked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			ubal,
			utils.Address(pk),
			amount,
		)
	}
	return SetBalance(ctx, db, pk, nbal, lbal)
}

func LockBalance(ctx context.Context, db chain.Database, pk crypto.PublicKey, amount uint64) error {
	ubal, lbal, err := GetBalance(ctx, db, pk)
	if err != nil {
		return err
	}
	nubal, err := smath.Sub(ubal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract unlocked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			ubal,
			utils.Address(pk),
			amount,
		)
	}
	nlbal, err := smath.Add64(lbal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not add locked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			lbal,
			utils.Address(pk),
			amount,
		)
	}
	return SetBalance(ctx, db, pk, nubal, nlbal)
}

func UnlockBalance(
	ctx context.Context,
	db chain.Database,
	pk crypto.PublicKey,
	amount uint64,
) error {
	ubal, lbal, err := GetBalance(ctx, db, pk)
	if err != nil {
		return err
	}
	nubal, err := smath.Add64(ubal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not add unlocked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			ubal,
			utils.Address(pk),
			amount,
		)
	}
	nlbal, err := smath.Sub(lbal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract locked bal=%d, addr=%v, amount=%d",
			ErrInvalidBalance,
			lbal,
			utils.Address(pk),
			amount,
		)
	}
	return SetBalance(ctx, db, pk, nubal, nlbal)
}

func GetContent(
	ctx context.Context,
	db chain.Database,
	contentID ids.ID,
) (crypto.PublicKey, uint64, bool, error) {
	k := PrefixContentKey(contentID)
	return innerGetContent(db.GetValue(ctx, k))
}

func GetContentFromState(
	ctx context.Context,
	f ReadState,
	contentID ids.ID,
) (crypto.PublicKey, uint64, bool, error) {
	values, errs := f(ctx, [][]byte{PrefixContentKey(contentID)})
	return innerGetContent(values[0], errs[0])
}

func innerGetContent(v []byte, err error) (crypto.PublicKey, uint64, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return crypto.PublicKey{}, 0, false, nil
	}
	if err != nil {
		return crypto.PublicKey{}, 0, false, err
	}
	var pk crypto.PublicKey
	copy(pk[:], v[:32])
	return pk, binary.BigEndian.Uint64(v[32:]), true, nil
}

func IndexContent(
	ctx context.Context,
	db chain.Database,
	contentID ids.ID,
	pk crypto.PublicKey,
	royalty uint64,
) error {
	if royalty == 0 {
		return ErrInsufficientTip
	}
	owner, _, exists, err := GetContent(ctx, db, contentID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf(
			"%w: owner=%s royalty=%d",
			ErrContentAlreadyExists,
			utils.Address(owner),
			royalty,
		)
	}
	return SetContent(ctx, db, contentID, pk, royalty)
}

func SetContent(
	ctx context.Context,
	db chain.Database,
	contentID ids.ID,
	owner crypto.PublicKey,
	royalty uint64,
) error {
	v := make([]byte, 40)
	copy(v, owner[:])
	binary.BigEndian.PutUint64(v[32:], royalty)
	return db.Insert(ctx, PrefixContentKey(contentID), v)
}

func UnindexContent(
	ctx context.Context,
	db chain.Database,
	contentID ids.ID,
	pk crypto.PublicKey,
) error {
	owner, _, exists, err := GetContent(ctx, db, contentID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrContentMissing
	}
	if owner != pk {
		return fmt.Errorf("%w: owner=%s", ErrWrongOwner, owner)
	}
	return db.Remove(ctx, PrefixContentKey(contentID))
}

func RewardSearcher(
	ctx context.Context,
	db chain.Database,
	contentID ids.ID,
	sender crypto.PublicKey,
) (crypto.PublicKey, error) {
	owner, royalty, exists, err := GetContent(ctx, db, contentID)
	if err != nil {
		return crypto.EmptyPublicKey, err
	}
	if !exists {
		// No tip to pay
		return crypto.EmptyPublicKey, nil
	}
	if err := SubUnlockedBalance(ctx, db, sender, royalty); err != nil {
		return crypto.EmptyPublicKey, err
	}
	if _, err := AddUnlockedBalance(ctx, db, owner, royalty, false); err != nil {
		return crypto.EmptyPublicKey, err
	}
	return owner, nil
}

// [accountPrefix] + [actor] + [signer]
func PrefixPermissionsKey(actor crypto.PublicKey, signer crypto.PublicKey) (k []byte) {
	k = make([]byte, 1+ed25519.PublicKeySize*2)
	k[0] = permissionsPrefix
	copy(k[1:], actor[:])
	copy(k[1+crypto.PublicKeyLen:], signer[:])
	return
}

func GetPermissions(
	ctx context.Context,
	db chain.Database,
	actor crypto.PublicKey,
	signer crypto.PublicKey,
) (uint8, uint8, error) {
	k := PrefixPermissionsKey(actor, signer)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return v[0], v[1], nil
}

func SetPermissions(
	ctx context.Context,
	db chain.Database,
	actor crypto.PublicKey,
	signer crypto.PublicKey,
	actionPerms uint8,
	miscPerms uint8,
) error {
	k := PrefixPermissionsKey(actor, signer)
	return db.Insert(ctx, k, []byte{actionPerms, miscPerms})
}

func DeletePermissions(
	ctx context.Context,
	db chain.Database,
	actor crypto.PublicKey,
	signer crypto.PublicKey,
) error {
	k := PrefixPermissionsKey(actor, signer)
	return db.Remove(ctx, k)
}
