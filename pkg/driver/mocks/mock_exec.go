// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

type MockExec struct {
	ctrl     *gomock.Controller
	recorder *MockExecMockRecorder
}

type MockExecMockRecorder struct {
	mock *MockExec
}

// NewMockExec creates a new mock instance
func NewMockExec(ctrl *gomock.Controller) *MockExec {
	mock := &MockExec{ctrl: ctrl}
	mock.recorder = &MockExecMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockExec) EXPECT() *MockExecMockRecorder {
	return m.recorder
}

// CleanSubPaths mocks base method
func (m *MockExec) Run(cmd string, args ...string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", cmd, args)
	ret0, _ := ret[0].(error)
	return nil, ret0
}

// MakeDir indicates an expected call of MakeDir
func (mr *MockExecMockRecorder) Run(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockExec)(nil).Run), arg0, arg1)
}
