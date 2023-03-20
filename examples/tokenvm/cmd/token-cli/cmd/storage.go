// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	defaultPrefix = 0x0
	keyPrefix     = 0x1
	chainPrefix   = 0x2

	defaultKeyKey   = "key"
	defaultChainKey = "chain"
)

func StoreDefault(key string, value []byte) error {
	k := make([]byte, 1+len(key))
	k[0] = defaultPrefix
	copy(k[1:], []byte(key))
	return db.Put(k, value)
}

func GetDefault(key string) ([]byte, error) {
	k := make([]byte, 1+len(key))
	k[0] = defaultPrefix
	copy(k[1:], []byte(key))
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func StoreKey(privateKey crypto.PrivateKey) error {
	publicKey := privateKey.PublicKey()
	k := make([]byte, 1+crypto.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	has, err := db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return ErrDuplicate
	}
	return db.Put(k, privateKey[:])
}

func GetKey(publicKey crypto.PublicKey) (crypto.PrivateKey, error) {
	k := make([]byte, 1+crypto.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return crypto.EmptyPrivateKey, nil
	}
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	return crypto.PrivateKey(v), nil
}

func GetKeys() ([]crypto.PrivateKey, error) {
	iter := db.NewIteratorWithPrefix([]byte{keyPrefix})
	defer iter.Release()

	privateKeys := []crypto.PrivateKey{}
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		privateKeys = append(privateKeys, crypto.PrivateKey(iter.Value()))
	}
	return privateKeys, iter.Error()
}

func StoreChain(chainID ids.ID, rpc string) error {
	k := make([]byte, 1+consts.IDLen*2)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])
	brpc := []byte(rpc)
	rpcID := utils.ToID(brpc)
	copy(k[1+consts.IDLen:], rpcID[:])
	has, err := db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return ErrDuplicate
	}
	return db.Put(k, brpc)
}

func GetChain(chainID ids.ID) ([]string, error) {
	k := make([]byte, 1+consts.IDLen)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])

	rpcs := []string{}
	iter := db.NewIteratorWithPrefix(k)
	defer iter.Release()
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		rpcs = append(rpcs, string(iter.Value()))
	}
	return rpcs, iter.Error()
}

func GetChains() (map[ids.ID][]string, error) {
	iter := db.NewIteratorWithPrefix([]byte{chainPrefix})
	defer iter.Release()

	chains := map[ids.ID][]string{}
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		k := iter.Key()
		chainID := ids.ID(k[1 : 1+consts.IDLen])
		rpcs, ok := chains[chainID]
		if !ok {
			rpcs = []string{}
		}
		rpcs = append(rpcs, string(iter.Value()))
		chains[chainID] = rpcs
	}
	return chains, iter.Error()
}

func DeleteChains() ([]ids.ID, error) {
	chains, err := GetChains()
	if err != nil {
		return nil, err
	}
	chainIDs := make([]ids.ID, 0, len(chains))
	for chainID, rpcs := range chains {
		for _, rpc := range rpcs {
			k := make([]byte, 1+consts.IDLen*2)
			k[0] = chainPrefix
			copy(k[1:], chainID[:])
			brpc := []byte(rpc)
			rpcID := utils.ToID(brpc)
			copy(k[1+consts.IDLen:], rpcID[:])
			if err := db.Delete(k); err != nil {
				return nil, err
			}
		}
		chainIDs = append(chainIDs, chainID)
	}
	return chainIDs, nil
}
