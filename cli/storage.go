// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"errors"
	"fmt"

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

func (h *Handler) StoreDefault(key string, value []byte) error {
	k := make([]byte, 1+len(key))
	k[0] = defaultPrefix
	copy(k[1:], []byte(key))
	return h.db.Put(k, value)
}

func (h *Handler) GetDefault(key string) ([]byte, error) {
	k := make([]byte, 1+len(key))
	k[0] = defaultPrefix
	copy(k[1:], []byte(key))
	v, err := h.db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (h *Handler) StoreDefaultChain(chainID ids.ID) error {
	return h.StoreDefault(defaultChainKey, chainID[:])
}

func (h *Handler) GetDefaultChain() (ids.ID, []string, error) {
	v, err := h.GetDefault(defaultChainKey)
	if err != nil {
		return ids.Empty, nil, err
	}
	if len(v) == 0 {
		return ids.Empty, nil, ErrNoChains
	}
	chainID := ids.ID(v)
	uris, err := h.GetChain(chainID)
	if err != nil {
		return ids.Empty, nil, err
	}
	utils.Outf("{{yellow}}chainID:{{/}} %s\n", chainID)
	return chainID, uris, nil
}

func (h *Handler) StoreKey(privateKey crypto.PrivateKey) error {
	publicKey := privateKey.PublicKey()
	k := make([]byte, 1+crypto.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	has, err := h.db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return ErrDuplicate
	}
	return h.db.Put(k, privateKey[:])
}

func (h *Handler) GetKey(publicKey crypto.PublicKey) (crypto.PrivateKey, error) {
	k := make([]byte, 1+crypto.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
	v, err := h.db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return crypto.EmptyPrivateKey, nil
	}
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	return crypto.PrivateKey(v), nil
}

func (h *Handler) GetKeys() ([]crypto.PrivateKey, error) {
	iter := h.db.NewIteratorWithPrefix([]byte{keyPrefix})
	defer iter.Release()

	privateKeys := []crypto.PrivateKey{}
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		privateKeys = append(privateKeys, crypto.PrivateKey(iter.Value()))
	}
	return privateKeys, iter.Error()
}

func (h *Handler) StoreDefaultKey(pk crypto.PublicKey) error {
	return h.StoreDefault(defaultKeyKey, pk[:])
}

func (h *Handler) GetDefaultKey() (crypto.PrivateKey, error) {
	v, err := h.GetDefault(defaultKeyKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	if len(v) == 0 {
		return crypto.EmptyPrivateKey, ErrNoKeys
	}
	publicKey := crypto.PublicKey(v)
	priv, err := h.GetKey(publicKey)
	if err != nil {
		return crypto.EmptyPrivateKey, err
	}
	utils.Outf("{{yellow}}address:{{/}} %s\n", h.c.Address(publicKey))
	return priv, nil
}

func (h *Handler) StoreChain(chainID ids.ID, rpc string) error {
	k := make([]byte, 1+consts.IDLen*2)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])
	brpc := []byte(rpc)
	rpcID := utils.ToID(brpc)
	copy(k[1+consts.IDLen:], rpcID[:])
	has, err := h.db.Has(k)
	if err != nil {
		return err
	}
	if has {
		return ErrDuplicate
	}
	return h.db.Put(k, brpc)
}

func (h *Handler) GetChain(chainID ids.ID) ([]string, error) {
	k := make([]byte, 1+consts.IDLen)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])

	rpcs := []string{}
	iter := h.db.NewIteratorWithPrefix(k)
	defer iter.Release()
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		rpcs = append(rpcs, string(iter.Value()))
	}
	return rpcs, iter.Error()
}

func (h *Handler) GetChains() (map[ids.ID][]string, error) {
	iter := h.db.NewIteratorWithPrefix([]byte{chainPrefix})
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

func (h *Handler) DeleteChains() ([]ids.ID, error) {
	chains, err := h.GetChains()
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
			if err := h.db.Delete(k); err != nil {
				return nil, err
			}
		}
		chainIDs = append(chainIDs, chainID)
	}
	return chainIDs, nil
}

func (h *Handler) CloseDatabase() error {
	if h.db == nil {
		return nil
	}
	if err := h.db.Close(); err != nil {
		return fmt.Errorf("unable to close database: %w", err)
	}
	// Allow DB to be closed multiple times
	h.db = nil
	return nil
}
