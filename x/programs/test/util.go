// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"os"
	"os/exec"
	"path/filepath"

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

type ProgramStore struct {
	ProgramsMap map[ids.ID]string
	AccountMap  map[codec.Address]ids.ID
}

func NewProgramStore() ProgramStore {
	return ProgramStore{
		ProgramsMap: make(map[ids.ID]string),
		AccountMap:  make(map[codec.Address]ids.ID)}
}

func (t ProgramStore) GetAccountProgram(_ context.Context, account codec.Address) (ids.ID, error) {
	if programID, ok := t.AccountMap[account]; ok {
		return programID, nil
	}
	return ids.Empty, nil
}

func (t ProgramStore) GetProgramBytes(_ context.Context, programID ids.ID) ([]byte, error) {
	programName, ok := t.ProgramsMap[programID]
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

func (t ProgramStore) NewAccountWithProgram(_ context.Context, programID ids.ID, _ []byte) (codec.Address, error) {
	account := codec.CreateAddress(0, programID)
	t.AccountMap[account] = programID
	return account, nil
}

func (t ProgramStore) SetAccountProgram(_ context.Context, account codec.Address, programID ids.ID) error {
	t.AccountMap[account] = programID
	return nil
}

type StateLoader struct {
	Mu state.Mutable
}

func (t StateLoader) GetProgramState(address codec.Address) state.Mutable {
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
