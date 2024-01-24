// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/hypersdk/chain (interfaces: Action)
//
// Generated by this command:
//
//	mockgen -package=chain -destination=chain/mock_action.go github.com/ava-labs/hypersdk/chain Action
//

// Package chain is a generated GoMock package.
package chain

import (
	context "context"
	reflect "reflect"

	ids "github.com/ava-labs/avalanchego/ids"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	codec "github.com/ava-labs/hypersdk/codec"
	state "github.com/ava-labs/hypersdk/state"
	gomock "go.uber.org/mock/gomock"
)

// MockAction is a mock of Action interface.
type MockAction struct {
	ctrl     *gomock.Controller
	recorder *MockActionMockRecorder
}

// MockActionMockRecorder is the mock recorder for MockAction.
type MockActionMockRecorder struct {
	mock *MockAction
}

// NewMockAction creates a new mock instance.
func NewMockAction(ctrl *gomock.Controller) *MockAction {
	mock := &MockAction{ctrl: ctrl}
	mock.recorder = &MockActionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAction) EXPECT() *MockActionMockRecorder {
	return m.recorder
}

// Execute mocks base method.
func (m *MockAction) Execute(arg0 context.Context, arg1 Rules, arg2 state.Mutable, arg3 int64, arg4 codec.Address, arg5 ids.ID, arg6 bool) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(uint64)
	ret2, _ := ret[2].([]byte)
	ret3, _ := ret[3].(*warp.UnsignedMessage)
	ret4, _ := ret[4].(error)
	return ret0, ret1, ret2, ret3, ret4
}

// Execute indicates an expected call of Execute.
func (mr *MockActionMockRecorder) Execute(arg0, arg1, arg2, arg3, arg4, arg5, arg6 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockAction)(nil).Execute), arg0, arg1, arg2, arg3, arg4, arg5, arg6)
}

// GetTypeID mocks base method.
func (m *MockAction) GetTypeID() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTypeID")
	ret0, _ := ret[0].(byte)
	return ret0
}

// GetTypeID indicates an expected call of GetTypeID.
func (mr *MockActionMockRecorder) GetTypeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTypeID", reflect.TypeOf((*MockAction)(nil).GetTypeID))
}

// Marshal mocks base method.
func (m *MockAction) Marshal(arg0 *codec.Packer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Marshal", arg0)
}

// Marshal indicates an expected call of Marshal.
func (mr *MockActionMockRecorder) Marshal(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Marshal", reflect.TypeOf((*MockAction)(nil).Marshal), arg0)
}

// MaxComputeUnits mocks base method.
func (m *MockAction) MaxComputeUnits(arg0 Rules) uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxComputeUnits", arg0)
	ret0, _ := ret[0].(uint64)
	return ret0
}

// MaxComputeUnits indicates an expected call of MaxComputeUnits.
func (mr *MockActionMockRecorder) MaxComputeUnits(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxComputeUnits", reflect.TypeOf((*MockAction)(nil).MaxComputeUnits), arg0)
}

// OutputsWarpMessage mocks base method.
func (m *MockAction) OutputsWarpMessage() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutputsWarpMessage")
	ret0, _ := ret[0].(bool)
	return ret0
}

// OutputsWarpMessage indicates an expected call of OutputsWarpMessage.
func (mr *MockActionMockRecorder) OutputsWarpMessage() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutputsWarpMessage", reflect.TypeOf((*MockAction)(nil).OutputsWarpMessage))
}

// Size mocks base method.
func (m *MockAction) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size.
func (mr *MockActionMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockAction)(nil).Size))
}

// StateKeys mocks base method.
func (m *MockAction) StateKeys(arg0 codec.Address, arg1 ids.ID) []state.Key {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateKeys", arg0, arg1)
	ret0, _ := ret[0].([]state.Key)
	return ret0
}

// StateKeys indicates an expected call of StateKeys.
func (mr *MockActionMockRecorder) StateKeys(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateKeys", reflect.TypeOf((*MockAction)(nil).StateKeys), arg0, arg1)
}

// StateKeysMaxChunks mocks base method.
func (m *MockAction) StateKeysMaxChunks() []uint16 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateKeysMaxChunks")
	ret0, _ := ret[0].([]uint16)
	return ret0
}

// StateKeysMaxChunks indicates an expected call of StateKeysMaxChunks.
func (mr *MockActionMockRecorder) StateKeysMaxChunks() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateKeysMaxChunks", reflect.TypeOf((*MockAction)(nil).StateKeysMaxChunks))
}

// ValidRange mocks base method.
func (m *MockAction) ValidRange(arg0 Rules) (int64, int64) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidRange", arg0)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(int64)
	return ret0, ret1
}

// ValidRange indicates an expected call of ValidRange.
func (mr *MockActionMockRecorder) ValidRange(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidRange", reflect.TypeOf((*MockAction)(nil).ValidRange), arg0)
}
