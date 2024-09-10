// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

type TestStateManager struct {
	ProgramsMap map[string][]byte
	AccountMap  map[codec.Address]string
	Balances    map[codec.Address]uint64
	Mu          state.Mutable
}

func (t TestStateManager) GetAccountProgram(_ context.Context, account codec.Address) (ProgramID, error) {
	if programID, ok := t.AccountMap[account]; ok {
		return ProgramID(programID), nil
	}
	return ids.Empty[:], nil
}

func (t TestStateManager) GetProgramBytes(_ context.Context, programID ProgramID) ([]byte, error) {
	programBytes, ok := t.ProgramsMap[string(programID)]
	if !ok {
		return nil, errors.New("couldn't find program")
	}

	return programBytes, nil
}

func (t TestStateManager) CompileProgram(programID ProgramID, programName string) ([]byte, error) {
	if err := test.CompileTest(programName); err != nil {
		return nil, err
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	programBytes, err := os.ReadFile(filepath.Join(dir, "/wasm32-unknown-unknown/debug/"+programName+".wasm"))
	if err != nil {
		return nil, err
	}

	return programBytes, nil
}

func (t TestStateManager) SetProgramBytes(programID ProgramID, programBytes []byte) {
	t.ProgramsMap[string(programID)] = programBytes
}

func (t TestStateManager) CompileAndSetProgram(programID ProgramID, programName string) error {
	programBytes, err := t.CompileProgram(programID, programName)
	if err != nil {
		return err
	}
	t.SetProgramBytes(programID, programBytes)
	return nil
}

func (t TestStateManager) NewAccountWithProgram(_ context.Context, programID ProgramID, _ []byte) (codec.Address, error) {
	account := codec.CreateAddress(0, ids.GenerateTestID())
	t.AccountMap[account] = string(programID)
	return account, nil
}

func (t TestStateManager) SetAccountProgram(_ context.Context, account codec.Address, programID ProgramID) error {
	t.AccountMap[account] = string(programID)
	return nil
}

func (t TestStateManager) GetBalance(_ context.Context, address codec.Address) (uint64, error) {
	if balance, ok := t.Balances[address]; ok {
		return balance, nil
	}
	return 0, nil
}

func (t TestStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
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

func (t TestStateManager) GetProgramState(address codec.Address) state.Mutable {
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

type testRuntime struct {
	Context      context.Context
	callContext  CallContext
	StateManager StateManager
}

func (t *testRuntime) WithStateManager(manager StateManager) *testRuntime {
	t.callContext = t.callContext.WithStateManager(manager)
	return t
}

func (t *testRuntime) WithActor(address codec.Address) *testRuntime {
	t.callContext = t.callContext.WithActor(address)
	return t
}

func (t *testRuntime) WithFunction(s string) *testRuntime {
	t.callContext = t.callContext.WithFunction(s)
	return t
}

func (t *testRuntime) WithProgram(address codec.Address) *testRuntime {
	t.callContext = t.callContext.WithProgram(address)
	return t
}

func (t *testRuntime) WithFuel(u uint64) *testRuntime {
	t.callContext = t.callContext.WithFuel(u)
	return t
}

func (t *testRuntime) WithParams(bytes []byte) *testRuntime {
	t.callContext = t.callContext.WithParams(bytes)
	return t
}

func (t *testRuntime) WithHeight(height uint64) *testRuntime {
	t.callContext = t.callContext.WithHeight(height)
	return t
}

func (t *testRuntime) WithActionID(actionID ids.ID) *testRuntime {
	t.callContext = t.callContext.WithActionID(actionID)
	return t
}

func (t *testRuntime) WithTimestamp(ts uint64) *testRuntime {
	t.callContext = t.callContext.WithTimestamp(ts)
	return t
}

func (t *testRuntime) WithValue(value uint64) *testRuntime {
	t.callContext = t.callContext.WithValue(value)
	return t
}

// AddProgram compiles [programName] and sets the bytes in the state manager
func (t *testRuntime) AddProgram(programID ProgramID, programName string) error {
	return t.StateManager.(TestStateManager).CompileAndSetProgram(programID, programName)
}

func (t *testRuntime) CallProgram(program codec.Address, function string, params ...interface{}) ([]byte, error) {
	return t.callContext.CallProgram(
		t.Context,
		&CallInfo{
			Program:      program,
			State:        t.StateManager,
			FunctionName: function,
			Params:       test.SerializeParams(params...),
		})
}

func newTestProgram(ctx context.Context, program string) (*testProgram, error) {
	id := ids.GenerateTestID()
	account := codec.CreateAddress(0, id)
	stringedID := string(id[:])
	testProgram := &testProgram{
		Runtime: &testRuntime{
			Context: ctx,
			callContext: NewRuntime(
				NewConfig(),
				logging.NoLog{}).WithDefaults(CallInfo{Fuel: 10000000}),
			StateManager: TestStateManager{
				ProgramsMap: map[string][]byte{},
				AccountMap:  map[codec.Address]string{account: stringedID},
				Balances:    map[codec.Address]uint64{},
				Mu:          test.NewTestDB(),
			},
		},
		Address: account,
	}
	err := testProgram.Runtime.AddProgram(ProgramID(stringedID), program)
	if err != nil {
		return nil, err
	}

	return testProgram, nil
}

type testProgram struct {
	Runtime *testRuntime
	Address codec.Address
}

func (t *testProgram) Call(function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Address,
		function,
		params...)
}

func (t *testProgram) WithStateManager(manager StateManager) *testProgram {
	t.Runtime = t.Runtime.WithStateManager(manager)
	return t
}

func (t *testProgram) WithActor(address codec.Address) *testProgram {
	t.Runtime = t.Runtime.WithActor(address)
	return t
}

func (t *testProgram) WithFunction(s string) *testProgram {
	t.Runtime = t.Runtime.WithFunction(s)
	return t
}

func (t *testProgram) WithProgram(address codec.Address) *testProgram {
	t.Runtime = t.Runtime.WithProgram(address)
	return t
}

func (t *testProgram) WithFuel(u uint64) *testProgram {
	t.Runtime = t.Runtime.WithFuel(u)
	return t
}

func (t *testProgram) WithParams(bytes []byte) *testProgram {
	t.Runtime = t.Runtime.WithParams(bytes)
	return t
}

func (t *testProgram) WithHeight(height uint64) *testProgram {
	t.Runtime = t.Runtime.WithHeight(height)
	return t
}

func (t *testProgram) WithActionID(actionID ids.ID) *testProgram {
	t.Runtime = t.Runtime.WithActionID(actionID)
	return t
}

func (t *testProgram) WithTimestamp(ts uint64) *testProgram {
	t.Runtime = t.Runtime.WithTimestamp(ts)
	return t
}

func (t *testProgram) WithValue(value uint64) *testProgram {
	t.Runtime = t.Runtime.WithValue(value)
	return t
}

func into[T any](data []byte) T {
	result, err := Deserialize[T](data)
	if err != nil {
		panic(err.Error())
	}
	return *result
}
