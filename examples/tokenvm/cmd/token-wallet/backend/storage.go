// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/pebble"
	hutils "github.com/ava-labs/hypersdk/utils"
)

const (
	keyPrefix         = 0x0
	assetsPrefix      = 0x1
	transactionPrefix = 0x2
	searchPrefix      = 0x3
	addressPrefix     = 0x4
	orderPrefix       = 0x5
)

type Storage struct {
	db database.Database
}

func OpenStorage(databasePath string) (*Storage, error) {
	db, _, err := pebble.New(databasePath, pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}
	return &Storage{db}, nil
}

func (s *Storage) StoreKey(privateKey ed25519.PrivateKey) error {
	has, err := s.db.Has([]byte{keyPrefix})
	if err != nil {
		return err
	}
	if has {
		return ErrDuplicate
	}
	return s.db.Put([]byte{keyPrefix}, privateKey[:])
}

func (s *Storage) GetKey() (ed25519.PrivateKey, error) {
	v, err := s.db.Get([]byte{keyPrefix})
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPrivateKey, nil
	}
	if err != nil {
		return ed25519.EmptyPrivateKey, err
	}
	return ed25519.PrivateKey(v), nil
}

func (s *Storage) StoreAsset(assetID ids.ID, owned bool) error {
	k := make([]byte, 1+ids.IDLen)
	k[0] = assetsPrefix
	copy(k[1:], assetID[:])
	v := []byte{0x0}
	if owned {
		v = []byte{0x1}
	}
	return s.db.Put(k, v)
}

func (s *Storage) HasAsset(assetID ids.ID) (bool, error) {
	k := make([]byte, 1+ids.IDLen)
	k[0] = assetsPrefix
	copy(k[1:], assetID[:])
	return s.db.Has(k)
}

func (s *Storage) GetAssets() ([]ids.ID, []bool, error) {
	iter := s.db.NewIteratorWithPrefix([]byte{assetsPrefix})
	defer iter.Release()

	assets := []ids.ID{}
	owned := []bool{}
	for iter.Next() {
		assets = append(assets, ids.ID(iter.Key()[1:]))
		owned = append(owned, iter.Value()[0] == 0x1)
	}
	return assets, owned, iter.Error()
}

func (s *Storage) StoreTransaction(tx *TransactionInfo) error {
	txID, err := ids.FromString(tx.ID)
	if err != nil {
		return err
	}
	inverseTime := consts.MaxUint64 - uint64(time.Now().UnixMilli())
	k := make([]byte, 1+consts.Uint64Len+ids.IDLen)
	k[0] = transactionPrefix
	binary.BigEndian.PutUint64(k[1:], inverseTime)
	copy(k[1+consts.Uint64Len:], txID[:])
	b, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	return s.db.Put(k, b)
}

func (s *Storage) GetTransactions() ([]*TransactionInfo, error) {
	iter := s.db.NewIteratorWithPrefix([]byte{transactionPrefix})
	defer iter.Release()

	txs := []*TransactionInfo{}
	for iter.Next() {
		var tx TransactionInfo
		if err := json.Unmarshal(iter.Value(), &tx); err != nil {
			return nil, err
		}
		txs = append(txs, &tx)
	}
	return txs, iter.Error()
}

func (s *Storage) StoreAddress(address string, nickname string) error {
	addr, err := codec.ParseAddressBech32(tconsts.HRP, address)
	if err != nil {
		return err
	}
	k := make([]byte, 1+codec.AddressLen)
	k[0] = addressPrefix
	copy(k[1:], addr[:])
	return s.db.Put(k, []byte(nickname))
}

func (s *Storage) GetAddresses() ([]*AddressInfo, error) {
	iter := s.db.NewIteratorWithPrefix([]byte{addressPrefix})
	defer iter.Release()

	addresses := []*AddressInfo{}
	for iter.Next() {
		address := codec.Address(iter.Key()[1:])
		nickname := string(iter.Value())
		addresses = append(addresses, &AddressInfo{nickname, codec.MustAddressBech32(tconsts.HRP, address), fmt.Sprintf("%s [%s..%s]", nickname, address[:len(tconsts.HRP)+3], address[len(address)-3:])})
	}
	return addresses, iter.Error()
}

func (s *Storage) StoreSolution(solution *FaucetSearchInfo) error {
	solutionID := hutils.ToID([]byte(solution.Solution))
	inverseTime := consts.MaxUint64 - uint64(time.Now().UnixMilli())
	k := make([]byte, 1+consts.Uint64Len+ids.IDLen)
	k[0] = searchPrefix
	binary.BigEndian.PutUint64(k[1:], inverseTime)
	copy(k[1+consts.Uint64Len:], solutionID[:])
	b, err := json.Marshal(solution)
	if err != nil {
		return err
	}
	return s.db.Put(k, b)
}

func (s *Storage) GetSolutions() ([]*FaucetSearchInfo, error) {
	iter := s.db.NewIteratorWithPrefix([]byte{searchPrefix})
	defer iter.Release()

	fs := []*FaucetSearchInfo{}
	for iter.Next() {
		var f FaucetSearchInfo
		if err := json.Unmarshal(iter.Value(), &f); err != nil {
			return nil, err
		}
		fs = append(fs, &f)
	}
	return fs, iter.Error()
}

func (s *Storage) StoreOrder(orderID ids.ID) error {
	inverseTime := consts.MaxUint64 - uint64(time.Now().UnixMilli())
	k := make([]byte, 1+consts.Uint64Len+ids.IDLen)
	k[0] = orderPrefix
	binary.BigEndian.PutUint64(k[1:], inverseTime)
	copy(k[1+consts.Uint64Len:], orderID[:])
	return s.db.Put(k, nil)
}

func (s *Storage) GetOrders() ([]ids.ID, [][]byte, error) {
	iter := s.db.NewIteratorWithPrefix([]byte{orderPrefix})
	defer iter.Release()

	orders := []ids.ID{}
	keys := [][]byte{}
	for iter.Next() {
		k := iter.Key()
		orders = append(orders, ids.ID(k[1+consts.Uint64Len:]))
		keys = append(keys, k)
	}
	return orders, keys, iter.Error()
}

func (s *Storage) DeleteDBKey(k []byte) error {
	return s.db.Delete(k)
}

func (s *Storage) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}
	return nil
}
