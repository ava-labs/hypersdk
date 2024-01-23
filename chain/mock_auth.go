// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/hypersdk/chain (interfaces: Auth)
//
// Generated by this command:
//
//	mockgen -package=chain -destination=chain/mock_auth.go github.com/ava-labs/hypersdk/chain Auth
//

// Package chain is a generated GoMock package.
package chain

import (
	context "context"
	reflect "reflect"

	codec "github.com/ava-labs/hypersdk/codec"
	gomock "go.uber.org/mock/gomock"
	fees "github.com/ava-labs/hypersdk/fees"
)

// MockAuth is a mock of Auth interface.
type MockAuth struct {
	ctrl     *gomock.Controller
	recorder *MockAuthMockRecorder
}

// MockAuthMockRecorder is the mock recorder for MockAuth.
type MockAuthMockRecorder struct {
	mock *MockAuth
}

// NewMockAuth creates a new mock instance.
func NewMockAuth(ctrl *gomock.Controller) *MockAuth {
	mock := &MockAuth{ctrl: ctrl}
	mock.recorder = &MockAuthMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAuth) EXPECT() *MockAuthMockRecorder {
	return m.recorder
}

// Actor mocks base method.
func (m *MockAuth) Actor() codec.Address {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Actor")
	ret0, _ := ret[0].(codec.Address)
	return ret0
}

// Actor indicates an expected call of Actor.
func (mr *MockAuthMockRecorder) Actor() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Actor", reflect.TypeOf((*MockAuth)(nil).Actor))
}

// ComputeUnits mocks base method.
func (m *MockAuth) ComputeUnits(arg0 fees.Rules) uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ComputeUnits", arg0)
	ret0, _ := ret[0].(uint64)
	return ret0
}

// ComputeUnits indicates an expected call of ComputeUnits.
func (mr *MockAuthMockRecorder) ComputeUnits(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ComputeUnits", reflect.TypeOf((*MockAuth)(nil).ComputeUnits), arg0)
}

// GetTypeID mocks base method.
func (m *MockAuth) GetTypeID() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTypeID")
	ret0, _ := ret[0].(byte)
	return ret0
}

// GetTypeID indicates an expected call of GetTypeID.
func (mr *MockAuthMockRecorder) GetTypeID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTypeID", reflect.TypeOf((*MockAuth)(nil).GetTypeID))
}

// Marshal mocks base method.
func (m *MockAuth) Marshal(arg0 *codec.Packer) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Marshal", arg0)
}

// Marshal indicates an expected call of Marshal.
func (mr *MockAuthMockRecorder) Marshal(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Marshal", reflect.TypeOf((*MockAuth)(nil).Marshal), arg0)
}

// Size mocks base method.
func (m *MockAuth) Size() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Size")
	ret0, _ := ret[0].(int)
	return ret0
}

// Size indicates an expected call of Size.
func (mr *MockAuthMockRecorder) Size() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Size", reflect.TypeOf((*MockAuth)(nil).Size))
}

// Sponsor mocks base method.
func (m *MockAuth) Sponsor() codec.Address {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sponsor")
	ret0, _ := ret[0].(codec.Address)
	return ret0
}

// Sponsor indicates an expected call of Sponsor.
func (mr *MockAuthMockRecorder) Sponsor() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sponsor", reflect.TypeOf((*MockAuth)(nil).Sponsor))
}

// ValidRange mocks base method.
func (m *MockAuth) ValidRange(arg0 fees.Rules) (int64, int64) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidRange", arg0)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(int64)
	return ret0, ret1
}

// ValidRange indicates an expected call of ValidRange.
func (mr *MockAuthMockRecorder) ValidRange(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidRange", reflect.TypeOf((*MockAuth)(nil).ValidRange), arg0)
}

// Verify mocks base method.
func (m *MockAuth) Verify(arg0 context.Context, arg1 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Verify", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Verify indicates an expected call of Verify.
func (mr *MockAuthMockRecorder) Verify(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Verify", reflect.TypeOf((*MockAuth)(nil).Verify), arg0, arg1)
}
