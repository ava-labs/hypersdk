package cmd

import (
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
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

func StoreKey(publicKey crypto.PublicKey, privateKey crypto.PrivateKey) error {
	k := make([]byte, 1+crypto.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], publicKey[:])
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
	k := make([]byte, 1+consts.IDLen)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])
	return db.Put(k, []byte(rpc))
}

func GetChain(chainID ids.ID) (string, error) {
	k := make([]byte, 1+consts.IDLen)
	k[0] = chainPrefix
	copy(k[1:], chainID[:])
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func GetChains() ([]ids.ID, []string, error) {
	iter := db.NewIteratorWithPrefix([]byte{chainPrefix})
	defer iter.Release()

	chainIDs := []ids.ID{}
	rpcs := []string{}
	for iter.Next() {
		// It is safe to use these bytes directly because the database copies the
		// iterator value for us.
		chainIDs = append(chainIDs, ids.ID(iter.Value()))
		rpcs = append(rpcs, string(iter.Value()))
	}
	return chainIDs, rpcs, iter.Error()
}
