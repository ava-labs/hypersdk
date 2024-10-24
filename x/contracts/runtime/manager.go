package runtime

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ ContractManager = &ContractStateManager{}
	ErrUnknownAccount = errors.New("unknown account")
	contractKeyBytes  = []byte("contract")
)

const (
	contractPrefix = 0x0

	accountPrefix      = 0x1
	accountDataPrefix  = 0x0
	accountStatePrefix = 0x1

)

// default implementation of the ContractManager interface
type ContractStateManager struct {
	// this state mutable should have its own prefix that doesn't conflict with other state
	db state.Mutable
}

func NewContractStateManager(
	db state.Mutable,
) *ContractStateManager {
	return &ContractStateManager{
		db: db,
	}
}

func (p *ContractStateManager) GetContractState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, p.db)
}

// GetAccountContract grabs the associated id with [account]. The ID is the key mapping to the contractbytes
// Errors if there is no found account or an error fetching
func (p *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (ContractID, error) {
	contractID, exists, err := p.getAccountContract(ctx, account)
	if err != nil {
		return ids.Empty[:], err
	}
	if !exists {
		return ids.Empty[:], ErrUnknownAccount
	}
	return contractID[:], nil
}

// [contractID] -> [contractBytes]
func (p *ContractStateManager) GetContractBytes(ctx context.Context, contractID ContractID) ([]byte, error) {
	// TODO: take fee out of balance?
	contractBytes, err := p.db.GetValue(ctx, contractKey(contractID))
	if err != nil {
		return []byte{}, ErrUnknownAccount
	}

	return contractBytes, nil
}


func (p *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID ContractID, accountCreationData []byte) (codec.Address, error) {
	newID := sha256.Sum256(append(contractID, accountCreationData...))
	newAccount := codec.CreateAddress(0, newID)
	return newAccount, p.SetAccountContract(ctx, newAccount, contractID)
}

func (p *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID ContractID) error {
	return p.db.Insert(ctx, accountDataKey(account[:], contractKeyBytes), contractID)
}

// setContract stores [contract] at [contractID]
func (p *ContractStateManager) SetContractBytes(
	ctx context.Context,
	contractID ContractID,
	contract []byte,
) error {
	return p.db.Insert(ctx, contractKey(contractID[:]), contract)
}

func contractKey(key []byte) (k []byte) {
	k = make([]byte, 0, 1+len(key))
	k = append(k, contractPrefix)
	k = append(k, key...)
	return
}

// Creates a key an account balance key
func accountDataKey(account []byte, key []byte) (k []byte) {
	// accountPrefix + account + accountDataPrefix + key
	k = make([]byte, 0, 2+len(account)+len(key))
	k = append(k, accountPrefix)
	k = append(k, account...)
	k = append(k, accountDataPrefix)
	k = append(k, key...)
	return
}

func accountContractKey(account []byte) []byte {
	return accountDataKey(account, contractKeyBytes)
}

func (p *ContractStateManager) getAccountContract(ctx context.Context, account codec.Address) (ids.ID, bool, error) {
	v, err := p.db.GetValue(ctx, accountContractKey(account[:]))
	if errors.Is(err, database.ErrNotFound) {
		return ids.Empty, false, nil
	}
	if err != nil {
		return ids.Empty, false, err
	}
	return ids.ID(v[:ids.IDLen]), true, nil
}

// prefixed state
type PrefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func NewPrefixStateMutable(prefix []byte, inner state.Mutable) *PrefixedStateMutable {
	return &PrefixedStateMutable{inner: inner, prefix: prefix}
}

func (s *PrefixedStateMutable) prefixKey(key []byte) (k []byte) {
	k = make([]byte, len(s.prefix)+len(key))
	copy(k, s.prefix)
	copy(k[len(s.prefix):], key)
	return
}

func (s *PrefixedStateMutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return s.inner.GetValue(ctx, s.prefixKey(key))
}

func (s *PrefixedStateMutable) Insert(ctx context.Context, key []byte, value []byte) error {
	return s.inner.Insert(ctx, s.prefixKey(key), value)
}

func (s *PrefixedStateMutable) Remove(ctx context.Context, key []byte) error {
	return s.inner.Remove(ctx, s.prefixKey(key))
}

func newAccountPrefixedMutable(account codec.Address, mutable state.Mutable) state.Mutable {
	return &PrefixedStateMutable{inner: mutable, prefix: accountStateKey(account[:])}
}

func accountStateKey(key []byte) (k []byte) {
	k = make([]byte, 2+len(key))
	k[0] = accountPrefix
	copy(k[1:], key)
	k[len(k)-1] = accountStatePrefix
	return
}
