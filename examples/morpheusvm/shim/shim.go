// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"
	"io"
	"math/big"
	"slices"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	evm_state "github.com/ava-labs/subnet-evm/core/state"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/trie"
	"github.com/ava-labs/subnet-evm/trie/trienode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	_ evm_state.Database = (*databaseShim)(nil)
	_ evm_state.Trie     = (*trieShim)(nil)
)

type AccessSet map[common.Address]map[common.Hash]struct{}

func (a AccessSet) Add(addr common.Address) {
	if a[addr] == nil {
		a[addr] = make(map[common.Hash]struct{})
	}
}

func (a AccessSet) AddKey(addr common.Address, key common.Hash) {
	a.Add(addr)
	a[addr][key] = struct{}{}
}

type Tracer struct {
	Reads      AccessSet
	Writes     AccessSet
	CodeReads  map[common.Address]struct{}
	CodeWrites map[common.Address]struct{}
}

func NewTracer() *Tracer {
	return &Tracer{
		Reads:      make(AccessSet),
		Writes:     make(AccessSet),
		CodeReads:  make(map[common.Address]struct{}),
		CodeWrites: make(map[common.Address]struct{}),
	}
}

type rlpTracer struct {
	ReadAccounts  []common.Address
	WriteAccounts []common.Address
	ReadKeys      [][]common.Hash
	WriteKeys     [][]common.Hash
	CodeReads     []common.Address
	CodeWrites    []common.Address
}

func (t *Tracer) EncodeRLP(w io.Writer) error {
	var rlpTracer rlpTracer
	for addr := range t.Reads {
		rlpTracer.ReadAccounts = append(rlpTracer.ReadAccounts, addr)
	}
	for addr := range t.Writes {
		rlpTracer.WriteAccounts = append(rlpTracer.WriteAccounts, addr)
	}
	for addr := range t.CodeReads {
		rlpTracer.CodeReads = append(rlpTracer.CodeReads, addr)
	}
	for addr := range t.CodeWrites {
		rlpTracer.CodeWrites = append(rlpTracer.CodeWrites, addr)
	}
	slices.SortFunc(rlpTracer.ReadAccounts, common.Address.Cmp)
	slices.SortFunc(rlpTracer.WriteAccounts, common.Address.Cmp)
	slices.SortFunc(rlpTracer.CodeReads, common.Address.Cmp)
	slices.SortFunc(rlpTracer.CodeWrites, common.Address.Cmp)

	for _, addr := range rlpTracer.ReadAccounts {
		keys := make([]common.Hash, 0, len(t.Reads[addr]))
		for key := range t.Reads[addr] {
			keys = append(keys, key)
		}
		slices.SortFunc(keys, common.Hash.Cmp)
		rlpTracer.ReadKeys = append(rlpTracer.ReadKeys, keys)
	}
	for _, addr := range rlpTracer.WriteAccounts {
		keys := make([]common.Hash, 0, len(t.Writes[addr]))
		for key := range t.Writes[addr] {
			keys = append(keys, key)
		}
		slices.SortFunc(keys, common.Hash.Cmp)
		rlpTracer.WriteKeys = append(rlpTracer.WriteKeys, keys)
	}

	return rlp.Encode(w, rlpTracer)
}

func (t *Tracer) DecodeRLP(s *rlp.Stream) error {
	var rlpTracer rlpTracer
	if err := s.Decode(&rlpTracer); err != nil {
		return err
	}
	*t = *NewTracer()
	for i, addr := range rlpTracer.ReadAccounts {
		t.Reads.Add(addr)
		for _, key := range rlpTracer.ReadKeys[i] {
			t.Reads.AddKey(addr, key)
		}
	}
	for i, addr := range rlpTracer.WriteAccounts {
		t.Writes.Add(addr)
		for _, key := range rlpTracer.WriteKeys[i] {
			t.Writes.AddKey(addr, key)
		}
	}
	for _, addr := range rlpTracer.CodeReads {
		t.CodeReads[addr] = struct{}{}
	}
	for _, addr := range rlpTracer.CodeWrites {
		t.CodeWrites[addr] = struct{}{}
	}
	return nil
}

type databaseShim struct {
	ctx context.Context
	mu  state.Mutable

	tracer *Tracer
}

func NewStateDB(ctx context.Context, mu state.Mutable) *evm_state.StateDB {
	statedb, err := evm_state.New(common.Hash{}, NewDatabaseShim(ctx, mu), nil)
	if err != nil {
		panic(err) // This can never happen since OpenTrie will always succeed
	}
	return statedb
}

func NewStateDBWithTracer(ctx context.Context, mu state.Mutable, tracer *Tracer) *evm_state.StateDB {
	shim := NewDatabaseShim(ctx, mu)
	shim.tracer = tracer
	statedb, err := evm_state.New(common.Hash{}, shim, nil)
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
func (d *databaseShim) OpenStorageTrie(common.Hash, common.Address, common.Hash) (evm_state.Trie, error) {
	return &trieShim{d}, nil
}
func (d *databaseShim) ContractCode(addr common.Address, _ common.Hash) ([]byte, error) {
	if d.tracer != nil {
		d.tracer.CodeReads[addr] = struct{}{}
	}
	return storage.GetCode(d.ctx, d.mu, addr)
}
func (d *databaseShim) ContractCodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := d.ContractCode(addr, codeHash)
	return len(code), err
}
func (*databaseShim) CopyTrie(t evm_state.Trie) evm_state.Trie { return t }
func (*databaseShim) DiskDB() ethdb.KeyValueStore              { panic("unimplemented") }
func (*databaseShim) TrieDB() *trie.Database                   { panic("unimplemented") }

type trieShim struct {
	d *databaseShim
}

func (*trieShim) GetKey([]byte) []byte { panic("unimplemented") }
func (t *trieShim) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	if t.d.tracer != nil {
		t.d.tracer.Reads.AddKey(addr, common.BytesToHash(key))
	}
	return storage.GetStorage(t.d.ctx, t.d.mu, addr, key)
}
func (t *trieShim) GetAccount(address common.Address) (*types.StateAccount, error) {
	if t.d.tracer != nil {
		t.d.tracer.Reads.Add(address)
	}
	// TODO: consolidate account & balance into a single storage entry
	var account types.StateAccount
	codecAddr := storage.BytesToAddress(address[:])
	balance, err := storage.GetBalance(t.d.ctx, t.d.mu, codecAddr)
	if err != nil {
		return nil, err
	}

	bytes, err := storage.GetAccount(t.d.ctx, t.d.mu, address)
	if err != nil {
		return nil, err
	}
	if len(bytes) > 0 {
		if err := rlp.DecodeBytes(bytes, &account); err != nil {
			return nil, err
		}
	}
	account.Balance = new(big.Int).SetUint64(balance)
	return &account, nil
}
func (t *trieShim) UpdateStorage(addr common.Address, key, value []byte) error {
	if t.d.tracer != nil {
		t.d.tracer.Writes.AddKey(addr, common.BytesToHash(key))
		return nil
	}
	return storage.SetStorage(t.d.ctx, t.d.mu, addr, key, value)
}
func (t *trieShim) UpdateAccount(address common.Address, account *types.StateAccount) error {
	if t.d.tracer != nil {
		t.d.tracer.Writes.Add(address)
		return nil
	}
	bytes, err := rlp.EncodeToBytes(account)
	if err != nil {
		return err
	}
	// TODO: consolidate account & balance into a single storage entry
	codecAddr := storage.BytesToAddress(address[:])
	if err := storage.SetBalance(t.d.ctx, t.d.mu, codecAddr, account.Balance.Uint64()); err != nil {
		return err
	}
	return storage.SetAccount(t.d.ctx, t.d.mu, address, bytes)
}
func (t *trieShim) UpdateContractCode(address common.Address, _ common.Hash, code []byte) error {
	if t.d.tracer != nil {
		t.d.tracer.CodeWrites[address] = struct{}{}
		return nil
	}
	return storage.SetCode(t.d.ctx, t.d.mu, address, code)
}
func (t *trieShim) DeleteStorage(addr common.Address, key []byte) error {
	if t.d.tracer != nil {
		t.d.tracer.Writes.AddKey(addr, common.BytesToHash(key))
		return nil
	}
	return storage.DeleteStorage(t.d.ctx, t.d.mu, addr, key)
}
func (t *trieShim) DeleteAccount(address common.Address) error {
	if t.d.tracer != nil {
		t.d.tracer.Writes.Add(address)
		return nil
	}
	return storage.DeleteAccount(t.d.ctx, t.d.mu, address)
}
func (*trieShim) Hash() common.Hash { return common.Hash{} }
func (*trieShim) Commit(bool) (common.Hash, *trienode.NodeSet, error) {
	panic("unimplemented")
}
func (*trieShim) NodeIterator([]byte) (trie.NodeIterator, error) { panic("unimplemented") }
func (*trieShim) Prove([]byte, ethdb.KeyValueWriter) error       { panic("unimplemented") }
