// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/hypersdk/vm (interfaces: Controller)
//
// Generated by this command:
//
//	mockgen -package=vm -destination=vm/mock_controller.go github.com/ava-labs/hypersdk/vm Controller
//

// Package vm is a generated GoMock package.
package vm

import (
	context "context"
	reflect "reflect"

	metrics "github.com/ava-labs/avalanchego/api/metrics"
	snow "github.com/ava-labs/avalanchego/snow"
	builder "github.com/ava-labs/hypersdk/builder"
	chain "github.com/ava-labs/hypersdk/chain"
	gossiper "github.com/ava-labs/hypersdk/gossiper"
	gomock "go.uber.org/mock/gomock"
)

// MockController is a mock of Controller interface.
type MockController struct {
	ctrl     *gomock.Controller
	recorder *MockControllerMockRecorder
}

// MockControllerMockRecorder is the mock recorder for MockController.
type MockControllerMockRecorder struct {
	mock *MockController
}

// NewMockController creates a new mock instance.
func NewMockController(ctrl *gomock.Controller) *MockController {
	mock := &MockController{ctrl: ctrl}
	mock.recorder = &MockControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockController) EXPECT() *MockControllerMockRecorder {
	return m.recorder
}

// Accepted mocks base method.
func (m *MockController) Accepted(arg0 context.Context, arg1 *chain.StatelessBlock) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Accepted", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Accepted indicates an expected call of Accepted.
func (mr *MockControllerMockRecorder) Accepted(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Accepted", reflect.TypeOf((*MockController)(nil).Accepted), arg0, arg1)
}

// Initialize mocks base method.
func (m *MockController) Initialize(arg0 *VM, arg1 *snow.Context, arg2 metrics.MultiGatherer, arg3, arg4, arg5 []byte) (Config, Genesis, builder.Builder, gossiper.Gossiper, Handlers, chain.ActionRegistry, chain.AuthRegistry, map[byte]AuthEngine, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Initialize", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(Config)
	ret1, _ := ret[1].(Genesis)
	ret2, _ := ret[2].(builder.Builder)
	ret3, _ := ret[3].(gossiper.Gossiper)
	ret4, _ := ret[4].(Handlers)
	ret5, _ := ret[5].(chain.ActionRegistry)
	ret6, _ := ret[6].(chain.AuthRegistry)
	ret7, _ := ret[7].(map[byte]AuthEngine)
	ret8, _ := ret[8].(error)
	return ret0, ret1, ret2, ret3, ret4, ret5, ret6, ret7, ret8
}

// Initialize indicates an expected call of Initialize.
func (mr *MockControllerMockRecorder) Initialize(arg0, arg1, arg2, arg3, arg4, arg5 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Initialize", reflect.TypeOf((*MockController)(nil).Initialize), arg0, arg1, arg2, arg3, arg4, arg5)
}

// Rejected mocks base method.
func (m *MockController) Rejected(arg0 context.Context, arg1 *chain.StatelessBlock) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rejected", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Rejected indicates an expected call of Rejected.
func (mr *MockControllerMockRecorder) Rejected(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rejected", reflect.TypeOf((*MockController)(nil).Rejected), arg0, arg1)
}

// Rules mocks base method.
func (m *MockController) Rules(arg0 int64) chain.Rules {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rules", arg0)
	ret0, _ := ret[0].(chain.Rules)
	return ret0
}

// Rules indicates an expected call of Rules.
func (mr *MockControllerMockRecorder) Rules(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rules", reflect.TypeOf((*MockController)(nil).Rules), arg0)
}

// Shutdown mocks base method.
func (m *MockController) Shutdown(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shutdown", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Shutdown indicates an expected call of Shutdown.
func (mr *MockControllerMockRecorder) Shutdown(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shutdown", reflect.TypeOf((*MockController)(nil).Shutdown), arg0)
}

// StateManager mocks base method.
func (m *MockController) StateManager() chain.StateManager {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateManager")
	ret0, _ := ret[0].(chain.StateManager)
	return ret0
}

// StateManager indicates an expected call of StateManager.
func (mr *MockControllerMockRecorder) StateManager() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateManager", reflect.TypeOf((*MockController)(nil).StateManager))
}
