package storage

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var (
	ErrUnknownAccount = errors.New("unknown account")
	ErrContractExists = errors.New("contract already exists")
)

const (
	AccountTypeID = 0
	MaxKeySize    = 36
	MaxContractSize = units.MiB * 4
)

var _ runtime.StateManager = (*ContractStateManager)(nil)

type ContractStateManager struct {
	state.Mutable
}

func (c *ContractStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
	return GetBalance(ctx, c, address)
}

func (c *ContractStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	_, err := SubBalance(ctx, c, from, amount)
	if err != nil {
		return err
	}
	_, err = AddBalance(ctx, c, to, amount, true)

	return err
}

func (c *ContractStateManager) GetContractState(address codec.Address) state.Mutable {
	return &prefixedStateMutable{
		inner:  c,
		prefix: accountStatePrefix(address),
	}
}

func (c *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (runtime.ContractID, error) {
	k := AccountContractIDKey(account)

	v, err := c.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return runtime.ContractID(ids.Empty[:]), ErrUnknownAccount
	}
	if err != nil {
		return runtime.ContractID(ids.Empty[:]), err
	}
	return runtime.ContractID(v[:ids.IDLen]), nil
}

func ContractBytesKey(contractID runtime.ContractID) (k []byte) {
	// contractBytePrefix -> contractID = bytes
	k = make([]byte, 0, 1+ids.IDLen)
	k = append(k, contractBytePrefix)
	k = append(k, contractID[:]...)
	k, _ = keys.Encode(k, MaxKeySize)
	return
}

func (c *ContractStateManager) SetContractBytes(ctx context.Context, contractID runtime.ContractID, contractBytes []byte) error {
	return c.Insert(ctx, ContractBytesKey(contractID), contractBytes)
}

func (c *ContractStateManager) GetContractBytes(ctx context.Context, contractID runtime.ContractID) ([]byte, error) {
	return c.GetValue(ctx, ContractBytesKey(contractID))
}

func GetAccountAddress(contractID runtime.ContractID, accountCreationData []byte) codec.Address {
	return codec.CreateAddress(AccountTypeID, sha256.Sum256(append(contractID[:], accountCreationData...)))
}

func (c *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID runtime.ContractID, accountCreationData []byte) (codec.Address, error) {
	// TODO: don't we need to generate a random address here? where should we get the randomness?
	// if we use the account creation data, someone could override previous accounts
	account := GetAccountAddress(contractID, accountCreationData)

	// check if the account already exists
	if _, err := c.GetValue(ctx, AccountContractIDKey(account)); err == nil {
		return codec.EmptyAddress, ErrContractExists
	}

	return account, c.SetAccountContract(ctx, account, contractID)
}

func AccountContractIDKey(account codec.Address) (k []byte) {
	// contractPrefix -> account -> id prefix = contractID
	k = make([]byte, 2+codec.AddressLen)
	k = append(k, contractPrefix)
	k = append(k, account[:]...)
	k = append(k, contractIDPrefix)
	k, _ = keys.Encode(k, MaxKeySize)
	return
}

func (c *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID runtime.ContractID) error {
	k := AccountContractIDKey(account)
	return c.Insert(ctx, k, contractID[:])
}

type prefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func (s *prefixedStateMutable) prefixKey(key []byte) (k []byte) {
	k = make([]byte, len(s.prefix)+len(key))
	copy(k, s.prefix)
	copy(k[len(s.prefix):], key)
	return
}

func (s *prefixedStateMutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return s.inner.GetValue(ctx, s.prefixKey(key))
}

func (s *prefixedStateMutable) Insert(ctx context.Context, key []byte, value []byte) error {
	return s.inner.Insert(ctx, s.prefixKey(key), value)
}

func (s *prefixedStateMutable) Remove(ctx context.Context, key []byte) error {
	return s.inner.Remove(ctx, s.prefixKey(key))
}

func accountStatePrefix(addr codec.Address) (k []byte) {
	// contractPrefix + account + contractStatePrefix
	k = make([]byte, 0, 2+codec.AddressLen)
	k = append(k, contractPrefix)
	k = append(k, addr[:]...)
	k = append(k, contractStatePrefix)
	return
}
