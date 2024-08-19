// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	hactions "github.com/ava-labs/hypersdk/actions"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

type ReadState func(context.Context, []string) ([][]byte, []error)

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
// 0x2/ (hypersdk-timestamp)
// 0x3/ (hypersdk-fee)
// 0x4/ (hypersdk-incoming warp)
// 0x5/ (hypersdk-outgoing warp)

const (
	// metaDB
	//
	// TODO: remove
	txPrefix = 0x0

	// stateDB
	balancePrefix      = 0x0
	heightPrefix       = 0x1
	pHeightPrefix      = 0x2
	timestampPrefix    = 0x3
	feePrefix          = 0x4
	incomingWarpPrefix = 0x5
	outgoingWarpPrefix = 0x6
	epochPrefix        = 0x7

	// anchor related prefixs, see hypersdk/actions
	// anchorRegistryPrefix = 0xf0
	// anchorPrefix = 0xf1
)

const BalanceChunks uint16 = 1

var (
	failureByte  = byte(0x0)
	successByte  = byte(0x1)
	heightKey    = string(heightPrefix)
	pHeightKey   = string(pHeightPrefix)
	timestampKey = string(timestampPrefix)
	feeKey       = string(feePrefix)
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
func BalanceKey(addr codec.Address) string {
	k := make([]byte, 1+codec.AddressLen+consts.Uint16Len)
	k[0] = balancePrefix
	copy(k[1:], addr[:])
	binary.BigEndian.PutUint16(k[1+codec.AddressLen:], BalanceChunks)
	return string(k)
}

// If locked is 0, then account does not exist
func GetBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) (uint64, error) {
	_, bal, _, err := getBalance(ctx, im, addr)
	return bal, err
}

func getBalance(
	ctx context.Context,
	im state.Immutable,
	addr codec.Address,
) (string, uint64, bool, error) {
	k := BalanceKey(addr)
	bal, exists, err := innerGetBalance(im.Get(ctx, k))
	return k, bal, exists, err
}

// Used to serve RPC queries
func GetBalanceFromState(
	ctx context.Context,
	f ReadState,
	addr codec.Address,
) (uint64, error) {
	k := BalanceKey(addr)
	values, errs := f(ctx, []string{k})
	bal, _, err := innerGetBalance(values[0], errs[0])
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
	mu state.Mutable,
	addr codec.Address,
	balance uint64,
) error {
	k := BalanceKey(addr)
	return setBalance(ctx, mu, k, balance)
}

func setBalance(
	ctx context.Context,
	mu state.Mutable,
	key string,
	balance uint64,
) error {
	return mu.Put(ctx, key, binary.BigEndian.AppendUint64(nil, balance))
}

func AddBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) error {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return err
	}
	nbal, err := smath.Add64(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not add balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			codec.MustAddressBech32(mconsts.HRP, addr),
			amount,
		)
	}
	return setBalance(ctx, mu, key, nbal)
}

func SubBalance(
	ctx context.Context,
	mu state.Mutable,
	addr codec.Address,
	amount uint64,
) error {
	key, bal, _, err := getBalance(ctx, mu, addr)
	if err != nil {
		return err
	}
	nbal, err := smath.Sub(bal, amount)
	if err != nil {
		return fmt.Errorf(
			"%w: could not subtract balance (bal=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			codec.MustAddressBech32(mconsts.HRP, addr),
			amount,
		)
	}
	if nbal == 0 {
		// If there is no balance left, we should delete the record instead of
		// setting it to 0.
		return mu.Delete(ctx, key)
	}
	return setBalance(ctx, mu, key, nbal)
}

func HeightKey() string {
	return heightKey
}

func PHeightKey() string {
	return pHeightKey
}

func TimestampKey() string {
	return timestampKey
}

func FeeKey() string {
	return feeKey
}

func IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) string {
	k := make([]byte, 1+consts.IDLen*2)
	k[0] = incomingWarpPrefix
	copy(k[1:], sourceChainID[:])
	copy(k[1+consts.IDLen:], msgID[:])
	return string(k)
}

func OutgoingWarpKeyPrefix(txID ids.ID) string {
	k := make([]byte, 1+consts.IDLen)
	k[0] = outgoingWarpPrefix
	copy(k[1:], txID[:])
	return string(k)
}

func EpochKey(epoch uint64) string {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = epochPrefix
	binary.BigEndian.PutUint64(k[1:], epoch)
	return string(k)
}

func AnchorRegistryKey() string {
	return hactions.AnchorRegistryKey()

	// // state key must >= 2 bytes
	// k := make([]byte, 1+consts.Uint16Len)
	// k[0] = anchorRegisteryPrefix
	// binary.BigEndian.PutUint16(k[1:], BalanceChunks) //TODO: update the BalanceChunks to AnchorChunks
	// return string(k)
}

func PackNamespaces(namespaces [][]byte) ([]byte, error) {
	return hactions.PackNamespaces(namespaces)
}

func UnpackNamespaces(raw []byte) ([][]byte, error) {
	return hactions.UnpackNamespaces(raw)
}

func GetAnchors(
	ctx context.Context,
	im state.Immutable,
) ([][]byte, []string, error) {
	_, namespaces, urls, _, err := getAnchors(ctx, im)
	return namespaces, urls, err
}

func getAnchors(
	ctx context.Context,
	im state.Immutable,
) (string, [][]byte, []string, bool, error) {
	k := AnchorRegistryKey()
	namespaces, exists, err := innerGetAnchors(im.Get(ctx, k))
	if err != nil {
		return "", nil, nil, false, err
	}

	urls := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		url, err := GetAnchor(ctx, im, ns)
		if err != nil {
			return "", nil, nil, false, err
		}
		urls = append(urls, url)
	}
	return k, namespaces, urls, exists, err
}

// Used to serve RPC queries
func GetAnchorsFromState(
	ctx context.Context,
	f ReadState,
) ([][]byte, []string, error) {
	k := AnchorRegistryKey()
	values, errs := f(ctx, []string{k})
	namespaces, _, err := innerGetAnchors(values[0], errs[0])
	if err != nil {
		return nil, nil, err
	}

	urls := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		k := AnchorKey(ns)
		values, errs := f(ctx, []string{k})
		url, _, err := innerGetAnchor(values[0], errs[0])
		if err != nil {
			return nil, nil, err
		}
		urls = append(urls, url)
	}
	return namespaces, urls, err
}

func innerGetAnchors(
	v []byte,
	err error,
) ([][]byte, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	namespaces, err := UnpackNamespaces(v)
	if err != nil {
		return nil, false, err
	}

	return namespaces, true, nil
}

func SetAnchors(
	ctx context.Context,
	mu state.Mutable,
	namespaces [][]byte,
) error {
	return setAnchors(ctx, mu, namespaces)
}

func setAnchors(
	ctx context.Context,
	mu state.Mutable,
	namespaces [][]byte,
) error {
	k := AnchorRegistryKey()
	packed, err := PackNamespaces(namespaces)
	if err != nil {
		return err
	}
	return mu.Put(ctx, k, packed)
}

func AnchorKey(namespace []byte) string {
	return hactions.AnchorKey(namespace)
	// k := make([]byte, 1+len(namespace)+consts.Uint16Len)
	// k[0] = anchorPrefix
	// copy(k[1:], namespace[:])
	// binary.BigEndian.PutUint16(k[1+len(namespace):], BalanceChunks) //TODO: update the BalanceChunks to AnchorChunks
	// return string(k)
}

// If locked is 0, then account does not exist
func GetAnchor(
	ctx context.Context,
	im state.Immutable,
	namespace []byte,
) (string, error) {
	_, anchor, _, err := getAnchor(ctx, im, namespace)
	return anchor, err
}

func getAnchor(
	ctx context.Context,
	im state.Immutable,
	namespace []byte,
) (string, string, bool, error) {
	k := AnchorKey(namespace)
	anchor, exists, err := innerGetAnchor(im.Get(ctx, k))
	return k, anchor, exists, err
}

// Used to serve RPC queries
func GetAnchorFromState(
	ctx context.Context,
	f ReadState,
	namespace []byte,
) (string, error) {
	k := AnchorKey(namespace)
	values, errs := f(ctx, []string{k})
	anchor, _, err := innerGetAnchor(values[0], errs[0])
	return anchor, err
}

func innerGetAnchor(
	v []byte,
	err error,
) (string, bool, error) {
	if errors.Is(err, database.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(v), true, nil
}

func SetAnchor(
	ctx context.Context,
	mu state.Mutable,
	namespace []byte,
	url string,
) error {
	k := AnchorKey(namespace)
	return setAnchor(ctx, mu, k, url)
}

func setAnchor(
	ctx context.Context,
	mu state.Mutable,
	key string,
	url string,
) error {
	urlBytes := []byte(url)
	return mu.Put(ctx, key, urlBytes)
}

func DelAnchor(
	ctx context.Context,
	mu state.Mutable,
	namespace []byte,
) error {
	k := AnchorKey(namespace)
	return delAnchor(ctx, mu, k)
}

func delAnchor(
	ctx context.Context,
	mu state.Mutable,
	key string,
) error {
	return mu.Delete(ctx, key)
}
