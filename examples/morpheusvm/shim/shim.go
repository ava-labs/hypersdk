// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	evm_state "github.com/ava-labs/subnet-evm/core/state"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/trie"
	"github.com/ava-labs/subnet-evm/trie/trienode"
	"github.com/ava-labs/subnet-evm/triedb"
	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

var (
	_ evm_state.Database = (*databaseShim)(nil)
	_ evm_state.Trie     = (*trieShim)(nil)
)

type databaseShim struct {
	ctx context.Context
	mu  state.Mutable
}

func NewStateDB(ctx context.Context, mu state.Mutable) *evm_state.StateDB {
	statedb, err := evm_state.New(common.Hash{}, NewDatabaseShim(ctx, mu), nil)
	if err != nil {
		panic(err) // This can never happen since OpenTrie will always succeed
	}
	return statedb
}

func NewDatabaseShim(ctx context.Context, mu state.Mutable) *databaseShim {
	return &databaseShim{ctx: ctx, mu: mu}
}

func (d *databaseShim) OpenTrie(common.Hash) (evm_state.Trie, error) {
	return &trieShim{d}, nil
}

func (d *databaseShim) OpenStorageTrie(root common.Hash, addr common.Address, hash common.Hash, _ evm_state.Trie) (evm_state.Trie, error) {
	return &trieShim{d}, nil
}

func (d *databaseShim) ContractCode(addr common.Address, _ common.Hash) ([]byte, error) {
	return storage.GetCode(d.ctx, d.mu, addr)
}

func (d *databaseShim) ContractCodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := d.ContractCode(addr, codeHash)
	return len(code), err
}
func (*databaseShim) CopyTrie(t evm_state.Trie) evm_state.Trie { return t }
func (*databaseShim) DiskDB() ethdb.KeyValueStore              { panic("unimplemented") }
func (*databaseShim) TrieDB() *triedb.Database                 { panic("unimplemented") }

type trieShim struct {
	d *databaseShim
}

func (*trieShim) GetKey([]byte) []byte { panic("unimplemented") }
func (t *trieShim) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	return storage.GetStorage(t.d.ctx, t.d.mu, addr, key)
}

func (t *trieShim) GetAccount(address common.Address) (*types.StateAccount, error) {
	// TODO: consolidate account & balance into a single storage entry
	var account types.StateAccount
	balance, err := storage.GetBalance(t.d.ctx, t.d.mu, address[:])
	if err != nil {
		return nil, err
	}

	bytes, err := storage.GetAccount(t.d.ctx, t.d.mu, address)
	if err != nil {
		return nil, err
	}
	p := codec.NewReader(bytes, len(bytes))
	if len(bytes) > 0 {
		unlimited := -1
		var buf []byte
		p.UnpackBytes(unlimited, false, &buf)
		account.Balance = uint256.NewInt(0).SetBytes(buf)
		account.Nonce = p.UnpackUint64(false)

		hashBuf := make([]byte, 32)
		p.UnpackFixedBytes(len(hashBuf), &hashBuf)
		copy(account.Root[:], hashBuf)

		p.UnpackBytes(unlimited, false, &account.CodeHash)
	}
	account.Balance = uint256.NewInt(0).SetUint64(balance)
	return &account, p.Err()
}

func (t *trieShim) UpdateStorage(addr common.Address, key, value []byte) error {
	// TODO: why is this copy necessary?
	value = common.CopyBytes(value)
	return storage.SetStorage(t.d.ctx, t.d.mu, addr, key, value)
}

func (t *trieShim) UpdateAccount(address common.Address, account *types.StateAccount) error {
	p := codec.NewWriter(0, consts.MaxInt)
	p.PackBytes(account.Balance.Bytes())
	p.PackUint64(account.Nonce)
	p.PackFixedBytes(account.Root[:])
	p.PackBytes(account.CodeHash)
	if err := p.Err(); err != nil {
		return err
	}
	// TODO: consolidate account & balance into a single storage entry
	if err := storage.SetBalance(t.d.ctx, t.d.mu, address[:], account.Balance.Uint64()); err != nil {
		return err
	}
	return storage.SetAccount(t.d.ctx, t.d.mu, address, p.Bytes())
}

func (t *trieShim) UpdateContractCode(address common.Address, _ common.Hash, code []byte) error {
	return storage.SetCode(t.d.ctx, t.d.mu, address, code)
}

func (t *trieShim) DeleteStorage(addr common.Address, key []byte) error {
	return storage.DeleteStorage(t.d.ctx, t.d.mu, addr, key)
}

func (t *trieShim) DeleteAccount(address common.Address) error {
	return storage.DeleteAccount(t.d.ctx, t.d.mu, address)
}
func (*trieShim) Hash() common.Hash { return common.Hash{} }
func (*trieShim) Commit(bool) (common.Hash, *trienode.NodeSet, error) {
	panic("unimplemented")
}
func (*trieShim) NodeIterator([]byte) (trie.NodeIterator, error) { panic("unimplemented") }
func (*trieShim) Prove([]byte, ethdb.KeyValueWriter) error       { panic("unimplemented") }
