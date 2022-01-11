// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juicedata/juicefs-csi-driver/pkg/juicefs (interfaces: Interface)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	config "github.com/juicedata/juicefs-csi-driver/pkg/config"
	juicefs "github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	mount "k8s.io/utils/mount"
)

// MockInterface is a mock of Interface interface.
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface.
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance.
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// AuthFs mocks base method.
func (m *MockInterface) AuthFs(arg0, arg1 map[string]string) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AuthFs", arg0, arg1)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AuthFs indicates an expected call of AuthFs.
func (mr *MockInterfaceMockRecorder) AuthFs(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AuthFs", reflect.TypeOf((*MockInterface)(nil).AuthFs), arg0, arg1)
}

// GetMountRefs mocks base method.
func (m *MockInterface) GetMountRefs(arg0 string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMountRefs", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMountRefs indicates an expected call of GetMountRefs.
func (mr *MockInterfaceMockRecorder) GetMountRefs(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMountRefs", reflect.TypeOf((*MockInterface)(nil).GetMountRefs), arg0)
}

// IsLikelyNotMountPoint mocks base method.
func (m *MockInterface) IsLikelyNotMountPoint(arg0 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLikelyNotMountPoint", arg0)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsLikelyNotMountPoint indicates an expected call of IsLikelyNotMountPoint.
func (mr *MockInterfaceMockRecorder) IsLikelyNotMountPoint(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLikelyNotMountPoint", reflect.TypeOf((*MockInterface)(nil).IsLikelyNotMountPoint), arg0)
}

// JfsCleanupMountPoint mocks base method.
func (m *MockInterface) JfsCleanupMountPoint(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JfsCleanupMountPoint", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// JfsCleanupMountPoint indicates an expected call of JfsCleanupMountPoint.
func (mr *MockInterfaceMockRecorder) JfsCleanupMountPoint(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JfsCleanupMountPoint", reflect.TypeOf((*MockInterface)(nil).JfsCleanupMountPoint), arg0)
}

// JfsMount mocks base method.
func (m *MockInterface) JfsMount(arg0, arg1 string, arg2, arg3 map[string]string, arg4 []string, arg5 bool) (juicefs.Jfs, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JfsMount", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(juicefs.Jfs)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// JfsMount indicates an expected call of JfsMount.
func (mr *MockInterfaceMockRecorder) JfsMount(arg0, arg1, arg2, arg3, arg4, arg5 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JfsMount", reflect.TypeOf((*MockInterface)(nil).JfsMount), arg0, arg1, arg2, arg3, arg4, arg5)
}

// JfsUnmount mocks base method.
func (m *MockInterface) JfsUnmount(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JfsUnmount", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// JfsUnmount indicates an expected call of JfsUnmount.
func (mr *MockInterfaceMockRecorder) JfsUnmount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JfsUnmount", reflect.TypeOf((*MockInterface)(nil).JfsUnmount), arg0)
}

// List mocks base method.
func (m *MockInterface) List() ([]mount.MountPoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List")
	ret0, _ := ret[0].([]mount.MountPoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockInterfaceMockRecorder) List() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockInterface)(nil).List))
}

// Mount mocks base method.
func (m *MockInterface) Mount(arg0, arg1, arg2 string, arg3 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Mount", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// Mount indicates an expected call of Mount.
func (mr *MockInterfaceMockRecorder) Mount(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Mount", reflect.TypeOf((*MockInterface)(nil).Mount), arg0, arg1, arg2, arg3)
}

// MountFs mocks base method.
func (m *MockInterface) MountFs(arg0, arg1 string, arg2 []string, arg3 *config.JfsSetting) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountFs", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MountFs indicates an expected call of MountFs.
func (mr *MockInterfaceMockRecorder) MountFs(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountFs", reflect.TypeOf((*MockInterface)(nil).MountFs), arg0, arg1, arg2, arg3)
}

// MountSensitive mocks base method.
func (m *MockInterface) MountSensitive(arg0, arg1, arg2 string, arg3, arg4 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MountSensitive", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// MountSensitive indicates an expected call of MountSensitive.
func (mr *MockInterfaceMockRecorder) MountSensitive(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MountSensitive", reflect.TypeOf((*MockInterface)(nil).MountSensitive), arg0, arg1, arg2, arg3, arg4)
}

// Unmount mocks base method.
func (m *MockInterface) Unmount(arg0 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unmount", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unmount indicates an expected call of Unmount.
func (mr *MockInterfaceMockRecorder) Unmount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unmount", reflect.TypeOf((*MockInterface)(nil).Unmount), arg0)
}

// Version mocks base method.
func (m *MockInterface) Version() ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Version")
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Version indicates an expected call of Version.
func (mr *MockInterfaceMockRecorder) Version() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockInterface)(nil).Version))
}
