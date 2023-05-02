// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/cli/svc_delete.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockwkldDeleter is a mock of wkldDeleter interface.
type MockwkldDeleter struct {
	ctrl     *gomock.Controller
	recorder *MockwkldDeleterMockRecorder
}

// MockwkldDeleterMockRecorder is the mock recorder for MockwkldDeleter.
type MockwkldDeleterMockRecorder struct {
	mock *MockwkldDeleter
}

// NewMockwkldDeleter creates a new mock instance.
func NewMockwkldDeleter(ctrl *gomock.Controller) *MockwkldDeleter {
	mock := &MockwkldDeleter{ctrl: ctrl}
	mock.recorder = &MockwkldDeleterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockwkldDeleter) EXPECT() *MockwkldDeleterMockRecorder {
	return m.recorder
}

// CleanResources mocks base method.
func (m *MockwkldDeleter) CleanResources(app, env, wkld string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CleanResources", app, env, wkld)
	ret0, _ := ret[0].(error)
	return ret0
}

// CleanResources indicates an expected call of CleanResources.
func (mr *MockwkldDeleterMockRecorder) CleanResources(app, env, wkld interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CleanResources", reflect.TypeOf((*MockwkldDeleter)(nil).CleanResources), app, env, wkld)
}
