// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount (interfaces: MntInterface)

// Package mocks is a generated GoMock package.
package mocks

import (
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	mount "k8s.io/utils/mount"
)

// MockMntInterface is a mock of MntInterface interface.
type MockMntInterface struct {
	ctrl     *gomock.Controller
	recorder *MockMntInterfaceMockRecorder
}

// MockMntInterfaceMockRecorder is the mock recorder for MockMntInterface.
type MockMntInterfaceMockRecorder struct {
	mock *MockMntInterface
}

// NewMockMntInterface creates a new mock instance.
func NewMockMntInterface(ctrl *gomock.Controller) *MockMntInterface {
	mock := &MockMntInterface{ctrl: ctrl}
	mock.recorder = &MockMntInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMntInterface) EXPECT() *MockMntInterfaceMockRecorder {
	return m.recorder
}

// AddRefOfMount mocks base method.
func (m *MockMntInterface) AddRefOfMount(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddRefOfMount", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddRefOfMount indicates an expected call of AddRefOfMount.
func (mr *MockMntInterfaceMockRecorder) AddRefOfMount(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddRefOfMount", reflect.TypeOf((*MockMntInterface)(nil).AddRefOfMount), arg0, arg1)
}

// GetMountRefs mocks base method.
func (m *MockMntInterface) GetMountRefs(arg0 string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMountRefs", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMountRefs indicates an expected call of GetMountRefs.
func (mr *MockMntInterfaceMockRecorder) GetMountRefs(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMountRefs", reflect.TypeOf((*MockMntInterface)(nil).GetMountRefs), arg0)
}

// IsLikelyNotMountPoint mocks base method.
func (m *MockMntInterface) IsLikelyNotMountPoint(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLikelyNotMountPoint", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsLikelyNotMountPoint indicates an expected call of IsLikelyNotMountPoint.
func (mr *MockMntInterfaceMockRecorder) IsLikelyNotMountPoint(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLikelyNotMountPoint", reflect.TypeOf((*MockMntInterface)(nil).IsLikelyNotMountPoint), arg0)
}

// JMount mocks base method.
func (m *MockMntInterface) JMount(arg0 *config.JfsSetting, arg1, arg2, arg3 string, arg4 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JMount", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// JMount indicates an expected call of JMount.
func (mr *MockMntInterfaceMockRecorder) JMount(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JMount", reflect.TypeOf((*MockMntInterface)(nil).JMount), arg0, arg1, arg2, arg3, arg4)
}

// JUmount mocks base method.
func (m *MockMntInterface) JUmount(arg0, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JUmount", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// JUmount indicates an expected call of JUmount.
func (mr *MockMntInterfaceMockRecorder) JUmount(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JUmount", reflect.TypeOf((*MockMntInterface)(nil).JUmount), arg0, arg1)
}

// List mocks base method.
func (m *MockMntInterface) List() ([]mount.MountPoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]mount.MountPoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockMntInterfaceMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockMntInterface)(nil).List))
}

// Mount mocks base method.
func (m *MockMntInterface) Mount(arg0, arg1, arg2 string, arg3 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mount", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mount indicates an expected call of Mount.
func (mr *MockMntInterfaceMockRecorder) Mount(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mount", reflect.TypeOf((*MockMntInterface)(nil).Mount), arg0, arg1, arg2, arg3)
}

// MountSensitive mocks base method.
func (m *MockMntInterface) MountSensitive(arg0, arg1, arg2 string, arg3, arg4 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitive", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitive indicates an expected call of MountSensitive.
func (mr *MockMntInterfaceMockRecorder) MountSensitive(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitive", reflect.TypeOf((*MockMntInterface)(nil).MountSensitive), arg0, arg1, arg2, arg3, arg4)
}

// Unmount mocks base method.
func (m *MockMntInterface) Unmount(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unmount", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unmount indicates an expected call of Unmount.
func (mr *MockMntInterfaceMockRecorder) Unmount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unmount", reflect.TypeOf((*MockMntInterface)(nil).Unmount), arg0)
}
