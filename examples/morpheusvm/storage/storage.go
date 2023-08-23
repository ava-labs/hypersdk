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
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
)

type ReadState func(context.Context, [][]byte) ([][]byte, []error)

// Metadata
// 0x0/ (tx)
//   -> [txID] => timestamp
//
// State
// / (height) => store in root
//   -> [heightPrefix] => height
// 0x0/ (balance)
//   -> [owner] => balance
// 0x1/ (hypersdk-height)
// 0x2/ (hypersdk-fee)
// 0x3/ (hypersdk-incoming warp)
// 0x4/ (hypersdk-outgoing warp)

const (
	// metaDB
	txPrefix = 0x0

	// stateDB
	balancePrefix      = 0x0
	heightPrefix       = 0x1
	feePrefix          = 0x2
	incomingWarpPrefix = 0x3
	outgoingWarpPrefix = 0x4
)

const BalanceChunks uint16 = 1

var (
	failureByte = byte(0x0)
	successByte = byte(0x1)
	heightKey   = []byte{heightPrefix}
	feeKey      = []byte{feePrefix}

	balanceKeyPool = sync.Pool{
		New: func() any {
			return make([]byte, 1+ed25519.PublicKeyLen+consts.Uint16Len)
		},
	}
)

// [txPrefix] + [txID]
func TxKey(id ids.ID) (k []byte) {
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
	units chain.Dimensions,
	fee uint64,
) error {
	k := TxKey(id)
	v := make([]byte, consts.Uint64Len+1+chain.DimensionsLen+consts.Uint64Len)
	binary.BigEndian.PutUint64(v, uint64(t))
	if success {
		v[consts.Uint64Len] = successByte
	} else {
		v[consts.Uint64Len] = failureByte
	}
	copy(v[consts.Uint64Len+1:], units.Bytes())
	binary.BigEndian.PutUint64(v[consts.Uint64Len+1+chain.DimensionsLen:], fee)
	return db.Put(k, v)
}

func GetTransaction(
	_ context.Context,
	db database.KeyValueReader,
	id ids.ID,
) (bool, int64, bool, chain.Dimensions, uint64, error) {
	k := TxKey(id)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, 0, false, chain.Dimensions{}, 0, nil
	}
	if err != nil {
		return false, 0, false, chain.Dimensions{}, 0, err
	}
	t := int64(binary.BigEndian.Uint64(v))
	success := true
	if v[consts.Uint64Len] == failureByte {
		success = false
	}
	d, err := chain.UnpackDimensions(v[consts.Uint64Len+1 : consts.Uint64Len+1+chain.DimensionsLen])
	if err != nil {
		return false, 0, false, chain.Dimensions{}, 0, err
	}
	fee := binary.BigEndian.Uint64(v[consts.Uint64Len+1+chain.DimensionsLen:])
	return true, t, success, d, fee, nil
}

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
	db chain.Database,
	pk ed25519.PublicKey,
) (uint64, error) {
	dbKey, bal, _, err := getBalance(ctx, db, pk)
	balanceKeyPool.Put(dbKey)
	return bal, err
}

func getBalance(
	ctx context.Context,
	db chain.Database,
	pk ed25519.PublicKey,
) ([]byte, uint64, bool, error) {
	k := BalanceKey(pk)
	bal, exists, err := innerGetBalance(db.GetValue(ctx, k))
	return k, bal, exists, err
}

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	pk ed25519.PublicKey,
) (uint64, error) {
	k := BalanceKey(pk)
	values, errs := f(ctx, [][]byte{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
	balanceKeyPool.Put(k)
	return bal, err
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
	db chain.Database,
	pk ed25519.PublicKey,
	balance uint64,
) error {
	k := BalanceKey(pk)
	return setBalance(ctx, db, k, balance)
}

func setBalance(
	ctx context.Context,
	db chain.Database,
	dbKey []byte,
	balance uint64,
) error {
	return db.Insert(ctx, dbKey, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	db chain.Database,
	pk ed25519.PublicKey,
	amount uint64,
	create bool,
) error {
	dbKey, bal, exists, err := getBalance(ctx, db, pk)
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
			utils.Address(pk),
			amount,
		)
	}
	return setBalance(ctx, db, dbKey, nbal)
}

func SubBalance(
	ctx context.Context,
	db chain.Database,
	pk ed25519.PublicKey,
	amount uint64,
) error {
	dbKey, bal, _, err := getBalance(ctx, db, pk)
	if err != nil {
		return err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			utils.Address(pk),
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return db.Remove(ctx, dbKey)
	}
	return setBalance(ctx, db, dbKey, nbal)
}

func HeightKey() (k []byte) {
	return heightKey
}

func FeeKey() (k []byte) {
	return feeKey
}

func IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen*2)
	k[0] = incomingWarpPrefix
	copy(k[1:], sourceChainID[:])
	copy(k[1+consts.IDLen:], msgID[:])
	return k
}

func OutgoingWarpKeyPrefix(txID ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = outgoingWarpPrefix
	copy(k[1:], txID[:])
	return k
}
