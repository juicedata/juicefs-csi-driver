// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juicedata/juicefs-csi-driver/pkg/juicefs (interfaces: Jfs)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	config "github.com/juicedata/juicefs-csi-driver/pkg/config"
)

// MockJfs is a mock of Jfs interface.
type MockJfs struct {
	ctrl     *gomock.Controller
	recorder *MockJfsMockRecorder
}

// MockJfsMockRecorder is the mock recorder for MockJfs.
type MockJfsMockRecorder struct {
	mock *MockJfs
}

// NewMockJfs creates a new mock instance.
func NewMockJfs(ctrl *gomock.Controller) *MockJfs {
	mock := &MockJfs{ctrl: ctrl}
	mock.recorder = &MockJfsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockJfs) EXPECT() *MockJfsMockRecorder {
	return m.recorder
}

// BindTarget mocks base method.
func (m *MockJfs) BindTarget(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BindTarget", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// BindTarget indicates an expected call of BindTarget.
func (mr *MockJfsMockRecorder) BindTarget(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BindTarget", reflect.TypeOf((*MockJfs)(nil).BindTarget), arg0, arg1, arg2)
}

// CreateVol mocks base method.
func (m *MockJfs) CreateVol(arg0 context.Context, arg1, arg2 string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateVol", arg0, arg1, arg2)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateVol indicates an expected call of CreateVol.
func (mr *MockJfsMockRecorder) CreateVol(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateVol", reflect.TypeOf((*MockJfs)(nil).CreateVol), arg0, arg1, arg2)
}

// GetBasePath mocks base method.
func (m *MockJfs) GetBasePath() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBasePath")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetBasePath indicates an expected call of GetBasePath.
func (mr *MockJfsMockRecorder) GetBasePath() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBasePath", reflect.TypeOf((*MockJfs)(nil).GetBasePath))
}

// GetSetting mocks base method.
func (m *MockJfs) GetSetting() *common.JfsSetting {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSetting")
	ret0, _ := ret[0].(*common.JfsSetting)
	return ret0
}

// GetSetting indicates an expected call of GetSetting.
func (mr *MockJfsMockRecorder) GetSetting() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSetting", reflect.TypeOf((*MockJfs)(nil).GetSetting))
}
