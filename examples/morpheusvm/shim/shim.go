// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"

	"github.com/ava-labs/hypersdk/state"
	evm_state "github.com/ava-labs/subnet-evm/core/state"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/trie"
	"github.com/ava-labs/subnet-evm/trie/trienode"
	"github.com/ava-labs/subnet-evm/triedb"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

var (
	_ evm_state.Database = (*DatabaseShim)(nil)
	_ evm_state.Trie     = (*trieShim)(nil)
)

type DatabaseShim struct {
	ctx context.Context // TODO: remove
	mu  state.Mutable
	err error
}

func NewStateDB(ctx context.Context, mu state.Mutable) (*evm_state.StateDB, *DatabaseShim) {
	shim := NewDatabaseShim(ctx, mu)
	statedb, err := evm_state.New(common.Hash{}, shim, nil)
	if err != nil {
		panic(err) // This can never happen since OpenTrie will always succeed
	}
	return statedb, shim
}

func NewDatabaseShim(ctx context.Context, mu state.Mutable) *DatabaseShim {
	return &DatabaseShim{ctx: ctx, mu: mu}
}

func (d *DatabaseShim) setError(err error) {
	if err != nil && d.err == nil {
		d.err = err
	}
}

func (d *DatabaseShim) Error() error {
	return d.err
}

func (d *DatabaseShim) OpenTrie(common.Hash) (evm_state.Trie, error) {
	return &trieShim{d}, nil
}

func (d *DatabaseShim) OpenStorageTrie(root common.Hash, addr common.Address, hash common.Hash, _ evm_state.Trie) (evm_state.Trie, error) {
	return &trieShim{d}, nil
}

func (d *DatabaseShim) ContractCode(addr common.Address, codeHash common.Hash) ([]byte, error) {
	codeBytes, err := storage.GetCode(d.ctx, d.mu, addr, codeHash)
	d.setError(err)
	return codeBytes, err
}

func (d *DatabaseShim) ContractCodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := d.ContractCode(addr, codeHash)
	d.setError(err)
	return len(code), err
}

func (d *DatabaseShim) GetNonce(addr common.Address) (uint64, error) {
	nonce, err := storage.GetNonce(d.ctx, d.mu, addr)
	d.setError(err)
	return nonce, err
}

func (*DatabaseShim) CopyTrie(evm_state.Trie) evm_state.Trie { panic("unimplemented") }
func (*DatabaseShim) DiskDB() ethdb.KeyValueStore            { panic("unimplemented") }
func (*DatabaseShim) TrieDB() *triedb.Database               { panic("unimplemented") }

type trieShim struct {
	d *DatabaseShim
}

func (*trieShim) GetKey([]byte) []byte { panic("unimplemented") }

func (t *trieShim) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	value, err := storage.GetStorage(t.d.ctx, t.d.mu, addr, key)
	t.d.setError(err)
	return value, err
}

func (t *trieShim) GetAccount(address common.Address) (*types.StateAccount, error) {
	bytes, err := storage.GetAccount(t.d.ctx, t.d.mu, address)
	t.d.setError(err)
	if err != nil {
		return nil, err
	}
	if len(bytes) > 0 {
		account, err := storage.DecodeAccount(bytes)
		if err != nil {
			return nil, err
		}
		return account, nil
	} else {
		return types.NewEmptyStateAccount(), nil
	}
}

func (t *trieShim) UpdateStorage(addr common.Address, key, value []byte) error {
	value = common.CopyBytes(value)
	err := storage.SetStorage(t.d.ctx, t.d.mu, addr, key, value)
	t.d.setError(err)
	return err
}

func (t *trieShim) UpdateAccount(address common.Address, account *types.StateAccount) error {
	encoded, err := storage.EncodeAccount(account)
	if err != nil {
		t.d.setError(err)
		return err
	}
	err = storage.SetAccount(t.d.ctx, t.d.mu, address, encoded)
	t.d.setError(err)
	return err
}

func (t *trieShim) UpdateContractCode(address common.Address, codeHash common.Hash, code []byte) error {
	err := storage.SetCode(t.d.ctx, t.d.mu, address, codeHash, code)
	t.d.setError(err)
	return err
}

func (t *trieShim) DeleteStorage(address common.Address, key []byte) error {
	err := storage.DeleteStorage(t.d.ctx, t.d.mu, address, key)
	t.d.setError(err)
	return err
}

func (t *trieShim) DeleteAccount(address common.Address) error {
	err := storage.DeleteAccount(t.d.ctx, t.d.mu, address)
	t.d.setError(err)
	return err
}
func (*trieShim) Hash() common.Hash                                   { return common.Hash{} }
func (*trieShim) Commit(bool) (common.Hash, *trienode.NodeSet, error) { panic("unimplemented") }
func (*trieShim) NodeIterator([]byte) (trie.NodeIterator, error)      { panic("unimplemented") }
func (*trieShim) Prove([]byte, ethdb.KeyValueWriter) error            { panic("unimplemented") }
