// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ava-labs/hypersdk/chain (interfaces: Rules)
//
// Generated by this command:
//
//	mockgen -package=chain -destination=chain/mock_rules.go github.com/ava-labs/hypersdk/chain Rules
//

// Package chain is a generated GoMock package.
package chain

import (
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/fees"

	"go.uber.org/mock/gomock"
)

// MockRules is a mock of Rules interface.
type MockRules struct {
	ctrl     *gomock.Controller
	recorder *MockRulesMockRecorder
}

// MockRulesMockRecorder is the mock recorder for MockRules.
type MockRulesMockRecorder struct {
	mock *MockRules
}

// NewMockRules creates a new mock instance.
func NewMockRules(ctrl *gomock.Controller) *MockRules {
	mock := &MockRules{ctrl: ctrl}
	mock.recorder = &MockRulesMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRules) EXPECT() *MockRulesMockRecorder {
	return m.recorder
}

// FetchCustom mocks base method.
func (m *MockRules) FetchCustom(arg0 string) (any, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchCustom", arg0)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// FetchCustom indicates an expected call of FetchCustom.
func (mr *MockRulesMockRecorder) FetchCustom(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchCustom", reflect.TypeOf((*MockRules)(nil).FetchCustom), arg0)
}

// GetBaseComputeUnits mocks base method.
func (m *MockRules) GetBaseComputeUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBaseComputeUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetBaseComputeUnits indicates an expected call of GetBaseComputeUnits.
func (mr *MockRulesMockRecorder) GetBaseComputeUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBaseComputeUnits", reflect.TypeOf((*MockRules)(nil).GetBaseComputeUnits))
}

// GetChainID mocks base method.
func (m *MockRules) GetChainID() ids.ID {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChainID")
	ret0, _ := ret[0].(ids.ID)
	return ret0
}

// GetChainID indicates an expected call of GetChainID.
func (mr *MockRulesMockRecorder) GetChainID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChainID", reflect.TypeOf((*MockRules)(nil).GetChainID))
}

// GetMaxActionsPerTx mocks base method.
func (m *MockRules) GetMaxActionsPerTx() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMaxActionsPerTx")
	ret0, _ := ret[0].(byte)
	return ret0
}

// GetMaxActionsPerTx indicates an expected call of GetMaxActionsPerTx.
func (mr *MockRulesMockRecorder) GetMaxActionsPerTx() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMaxActionsPerTx", reflect.TypeOf((*MockRules)(nil).GetMaxActionsPerTx))
}

// GetMaxBlockUnits mocks base method.
func (m *MockRules) GetMaxBlockUnits() fees.Dimensions {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMaxBlockUnits")
	ret0, _ := ret[0].(fees.Dimensions)
	return ret0
}

// GetMaxBlockUnits indicates an expected call of GetMaxBlockUnits.
func (mr *MockRulesMockRecorder) GetMaxBlockUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMaxBlockUnits", reflect.TypeOf((*MockRules)(nil).GetMaxBlockUnits))
}

// GetMaxOutputsPerAction mocks base method.
func (m *MockRules) GetMaxOutputsPerAction() byte {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMaxOutputsPerAction")
	ret0, _ := ret[0].(byte)
	return ret0
}

// GetMaxOutputsPerAction indicates an expected call of GetMaxOutputsPerAction.
func (mr *MockRulesMockRecorder) GetMaxOutputsPerAction() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMaxOutputsPerAction", reflect.TypeOf((*MockRules)(nil).GetMaxOutputsPerAction))
}

// GetMinBlockGap mocks base method.
func (m *MockRules) GetMinBlockGap() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMinBlockGap")
	ret0, _ := ret[0].(int64)
	return ret0
}

// GetMinBlockGap indicates an expected call of GetMinBlockGap.
func (mr *MockRulesMockRecorder) GetMinBlockGap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMinBlockGap", reflect.TypeOf((*MockRules)(nil).GetMinBlockGap))
}

// GetMinEmptyBlockGap mocks base method.
func (m *MockRules) GetMinEmptyBlockGap() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMinEmptyBlockGap")
	ret0, _ := ret[0].(int64)
	return ret0
}

// GetMinEmptyBlockGap indicates an expected call of GetMinEmptyBlockGap.
func (mr *MockRulesMockRecorder) GetMinEmptyBlockGap() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMinEmptyBlockGap", reflect.TypeOf((*MockRules)(nil).GetMinEmptyBlockGap))
}

// GetMinUnitPrice mocks base method.
func (m *MockRules) GetMinUnitPrice() fees.Dimensions {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMinUnitPrice")
	ret0, _ := ret[0].(fees.Dimensions)
	return ret0
}

// GetMinUnitPrice indicates an expected call of GetMinUnitPrice.
func (mr *MockRulesMockRecorder) GetMinUnitPrice() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMinUnitPrice", reflect.TypeOf((*MockRules)(nil).GetMinUnitPrice))
}

// GetNetworkID mocks base method.
func (m *MockRules) GetNetworkID() uint32 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNetworkID")
	ret0, _ := ret[0].(uint32)
	return ret0
}

// GetNetworkID indicates an expected call of GetNetworkID.
func (mr *MockRulesMockRecorder) GetNetworkID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNetworkID", reflect.TypeOf((*MockRules)(nil).GetNetworkID))
}

// GetSponsorStateKeysMaxChunks mocks base method.
func (m *MockRules) GetSponsorStateKeysMaxChunks() []uint16 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSponsorStateKeysMaxChunks")
	ret0, _ := ret[0].([]uint16)
	return ret0
}

// GetSponsorStateKeysMaxChunks indicates an expected call of GetSponsorStateKeysMaxChunks.
func (mr *MockRulesMockRecorder) GetSponsorStateKeysMaxChunks() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSponsorStateKeysMaxChunks", reflect.TypeOf((*MockRules)(nil).GetSponsorStateKeysMaxChunks))
}

// GetStorageKeyAllocateUnits mocks base method.
func (m *MockRules) GetStorageKeyAllocateUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageKeyAllocateUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageKeyAllocateUnits indicates an expected call of GetStorageKeyAllocateUnits.
func (mr *MockRulesMockRecorder) GetStorageKeyAllocateUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageKeyAllocateUnits", reflect.TypeOf((*MockRules)(nil).GetStorageKeyAllocateUnits))
}

// GetStorageKeyReadUnits mocks base method.
func (m *MockRules) GetStorageKeyReadUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageKeyReadUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageKeyReadUnits indicates an expected call of GetStorageKeyReadUnits.
func (mr *MockRulesMockRecorder) GetStorageKeyReadUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageKeyReadUnits", reflect.TypeOf((*MockRules)(nil).GetStorageKeyReadUnits))
}

// GetStorageKeyWriteUnits mocks base method.
func (m *MockRules) GetStorageKeyWriteUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageKeyWriteUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageKeyWriteUnits indicates an expected call of GetStorageKeyWriteUnits.
func (mr *MockRulesMockRecorder) GetStorageKeyWriteUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageKeyWriteUnits", reflect.TypeOf((*MockRules)(nil).GetStorageKeyWriteUnits))
}

// GetStorageValueAllocateUnits mocks base method.
func (m *MockRules) GetStorageValueAllocateUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageValueAllocateUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageValueAllocateUnits indicates an expected call of GetStorageValueAllocateUnits.
func (mr *MockRulesMockRecorder) GetStorageValueAllocateUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageValueAllocateUnits", reflect.TypeOf((*MockRules)(nil).GetStorageValueAllocateUnits))
}

// GetStorageValueReadUnits mocks base method.
func (m *MockRules) GetStorageValueReadUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageValueReadUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageValueReadUnits indicates an expected call of GetStorageValueReadUnits.
func (mr *MockRulesMockRecorder) GetStorageValueReadUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageValueReadUnits", reflect.TypeOf((*MockRules)(nil).GetStorageValueReadUnits))
}

// GetStorageValueWriteUnits mocks base method.
func (m *MockRules) GetStorageValueWriteUnits() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetStorageValueWriteUnits")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GetStorageValueWriteUnits indicates an expected call of GetStorageValueWriteUnits.
func (mr *MockRulesMockRecorder) GetStorageValueWriteUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetStorageValueWriteUnits", reflect.TypeOf((*MockRules)(nil).GetStorageValueWriteUnits))
}

// GetUnitPriceChangeDenominator mocks base method.
func (m *MockRules) GetUnitPriceChangeDenominator() fees.Dimensions {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUnitPriceChangeDenominator")
	ret0, _ := ret[0].(fees.Dimensions)
	return ret0
}

// GetUnitPriceChangeDenominator indicates an expected call of GetUnitPriceChangeDenominator.
func (mr *MockRulesMockRecorder) GetUnitPriceChangeDenominator() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUnitPriceChangeDenominator", reflect.TypeOf((*MockRules)(nil).GetUnitPriceChangeDenominator))
}

// GetValidityWindow mocks base method.
func (m *MockRules) GetValidityWindow() int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetValidityWindow")
	ret0, _ := ret[0].(int64)
	return ret0
}

// GetValidityWindow indicates an expected call of GetValidityWindow.
func (mr *MockRulesMockRecorder) GetValidityWindow() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetValidityWindow", reflect.TypeOf((*MockRules)(nil).GetValidityWindow))
}

// GetWindowTargetUnits mocks base method.
func (m *MockRules) GetWindowTargetUnits() fees.Dimensions {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWindowTargetUnits")
	ret0, _ := ret[0].(fees.Dimensions)
	return ret0
}

// GetWindowTargetUnits indicates an expected call of GetWindowTargetUnits.
func (mr *MockRulesMockRecorder) GetWindowTargetUnits() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWindowTargetUnits", reflect.TypeOf((*MockRules)(nil).GetWindowTargetUnits))
}
