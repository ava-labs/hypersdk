// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

func CompileTest(programName string) error {
	cmd := exec.Command("cargo", "build", "-p", programName, "--target", "wasm32-unknown-unknown", "--target-dir", "./")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	return nil
}

type StateManager struct {
	ProgramsMap map[string]string
	AccountMap  map[codec.Address]string
	Balances    map[codec.Address]uint64
	Mu          state.Mutable
}

func (t StateManager) GetAccountProgram(_ context.Context, account codec.Address) ([]byte, error) {
	if programID, ok := t.AccountMap[account]; ok {
		return []byte(programID), nil
	}
	return ids.Empty[:], nil
}

func (t StateManager) GetProgramBytes(_ context.Context, programID []byte) ([]byte, error) {
	programName, ok := t.ProgramsMap[string(programID)]
	if !ok {
		return nil, errors.New("couldn't find program")
	}
	if err := CompileTest(programName); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "/wasm32-unknown-unknown/debug/"+programName+".wasm"))
}

func (t StateManager) NewAccountWithProgram(_ context.Context, programID []byte, _ []byte) (codec.Address, error) {
	account := codec.CreateAddress(0, ids.GenerateTestID())
	t.AccountMap[account] = string(programID)
	return account, nil
}

func (t StateManager) SetAccountProgram(_ context.Context, account codec.Address, programID []byte) error {
	t.AccountMap[account] = string(programID)
	return nil
}

func (t StateManager) GetBalance(_ context.Context, address codec.Address) (uint64, error) {
	if balance, ok := t.Balances[address]; ok {
		return balance, nil
	}
	return 0, nil
}

func (t StateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	balance, err := t.GetBalance(ctx, from)
	if err != nil {
		return err
	}
	if balance < amount {
		return errors.New("insufficient balance")
	}
	t.Balances[from] -= amount
	t.Balances[to] += amount
	return nil
}

func (t StateManager) GetProgramState(address codec.Address) state.Mutable {
	return &prefixedState{address: address, inner: t.Mu}
}

var _ state.Mutable = (*prefixedState)(nil)

type prefixedState struct {
	address codec.Address
	inner   state.Mutable
}

func (p *prefixedState) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return p.inner.GetValue(ctx, prependAccountToKey(p.address, key))
}

func (p *prefixedState) Insert(ctx context.Context, key []byte, value []byte) error {
	return p.inner.Insert(ctx, prependAccountToKey(p.address, key), value)
}

func (p *prefixedState) Remove(ctx context.Context, key []byte) error {
	return p.inner.Remove(ctx, prependAccountToKey(p.address, key))
}

// prependAccountToKey makes the key relative to the account
func prependAccountToKey(account codec.Address, key []byte) []byte {
	result := make([]byte, len(account)+len(key)+1)
	copy(result, account[:])
	copy(result[len(account):], "/")
	copy(result[len(account)+1:], key)
	return result
}

func SerializeParams(params ...interface{}) []byte {
	if len(params) == 0 {
		return nil
	}
	results := make([][]byte, len(params))
	var err error
	for i, param := range params {
		results[i], err = borsh.Serialize(param)
		if err != nil {
			return nil
		}
	}
	return Flatten[byte](results...)
}

func Flatten[T any](slices ...[]T) []T {
	var size int
	for _, slice := range slices {
		size += len(slice)
	}

	result := make([]T, 0, size)
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}
