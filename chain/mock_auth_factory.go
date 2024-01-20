// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/hypersdk/chain (interfaces: AuthFactory)

// Package chain is a generated GoMock package.
package chain

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockAuthFactory is a mock of AuthFactory interface.
type MockAuthFactory struct {
	ctrl     *gomock.Controller
	recorder *MockAuthFactoryMockRecorder
}

// MockAuthFactoryMockRecorder is the mock recorder for MockAuthFactory.
type MockAuthFactoryMockRecorder struct {
	mock *MockAuthFactory
}

// NewMockAuthFactory creates a new mock instance.
func NewMockAuthFactory(ctrl *gomock.Controller) *MockAuthFactory {
	mock := &MockAuthFactory{ctrl: ctrl}
	mock.recorder = &MockAuthFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAuthFactory) EXPECT() *MockAuthFactoryMockRecorder {
	return m.recorder
}

// MaxUnits mocks base method.
func (m *MockAuthFactory) MaxUnits() (uint64, uint64) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MaxUnits")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(uint64)
	return ret0, ret1
}

// MaxUnits indicates an expected call of MaxUnits.
func (mr *MockAuthFactoryMockRecorder) MaxUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MaxUnits", reflect.TypeOf((*MockAuthFactory)(nil).MaxUnits))
}

// Sign mocks base method.
func (m *MockAuthFactory) Sign(arg0 []byte) (Auth, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sign", arg0)
	ret0, _ := ret[0].(Auth)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Sign indicates an expected call of Sign.
func (mr *MockAuthFactoryMockRecorder) Sign(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sign", reflect.TypeOf((*MockAuthFactory)(nil).Sign), arg0)
}
