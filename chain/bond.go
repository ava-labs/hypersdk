// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.

// See the file LICENSE for licensing terms.

package chain

import (
	"errors"
	"fmt"
	"sync"

	acodec "github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/fdsmr"
)

var _ fdsmr.Bonder[*Transaction] = (*Bonder)(nil)

type BondBalance struct {
	Pending uint32 `serialize:"true"`
	Max     uint32 `serialize:"true"`
}

// Bonder maintains state of account bond balances to limit the amount of
// pending transactions per account
type Bonder struct {
	codec acodec.Manager
	db    database.Database
	lock  sync.Mutex
}

// this needs to be thread-safe if it's called from the api
func (b *Bonder) SetMaxBalance(address codec.Address, i uint32) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	addressBytes := address[:]
	balance, err := b.getBalance(addressBytes)
	if err != nil {
		return err
	}

	balance.Max = i
	return b.putBalance(addressBytes, balance)
}

func (b *Bonder) Bond(tx *Transaction) (bool, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	address := tx.Sponsor()
	addressBytes := address[:]

	balance, err := b.getBalance(addressBytes)
	if err != nil {
		return false, err
	}

	if balance.Pending == balance.Max {
		return false, nil
	}

	balance.Pending++
	if err := b.putBalance(addressBytes, balance); err != nil {
		return false, err
	}

	return true, nil
}

func (b *Bonder) Unbond(tx *Transaction) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	address := tx.Sponsor()
	addressBytes := address[:]

	balance, err := b.getBalance(addressBytes)
	if err != nil {
		return err
	}

	balance.Pending--
	if err := b.putBalance(addressBytes, balance); err != nil {
		return err
	}

	return nil
}

func (b *Bonder) getBalance(address []byte) (BondBalance, error) {
	currentBytes, err := b.db.Get(address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return BondBalance{}, fmt.Errorf("failed to get bond balance")
	}

	p := &wrappers.Packer{Bytes: currentBytes}
	balance := BondBalance{}

	if err := codec.LinearCodec.UnmarshalFrom(p, &balance); err != nil {
		return BondBalance{}, fmt.Errorf("failed to unmarshal bond balance: %w", err)
	}

	return balance, nil
}

func (b *Bonder) putBalance(address []byte, balance BondBalance) error {
	p := &wrappers.Packer{}
	if err := codec.LinearCodec.MarshalInto(balance, p); err != nil {
		return fmt.Errorf("failed to marshal bond balance: %w", err)

	}

	if err := b.db.Put(address[:], p.Bytes); err != nil {
		return fmt.Errorf("failed to update bond balance: %w", err)
	}

	return nil
}
