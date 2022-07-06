// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/runner/jobrunner/jobrunner.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	cloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	gomock "github.com/golang/mock/gomock"
)

// MockStateMachineExecutor is a mock of StateMachineExecutor interface.
type MockStateMachineExecutor struct {
	ctrl     *gomock.Controller
	recorder *MockStateMachineExecutorMockRecorder
}

// MockStateMachineExecutorMockRecorder is the mock recorder for MockStateMachineExecutor.
type MockStateMachineExecutorMockRecorder struct {
	mock *MockStateMachineExecutor
}

// NewMockStateMachineExecutor creates a new mock instance.
func NewMockStateMachineExecutor(ctrl *gomock.Controller) *MockStateMachineExecutor {
	mock := &MockStateMachineExecutor{ctrl: ctrl}
	mock.recorder = &MockStateMachineExecutorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateMachineExecutor) EXPECT() *MockStateMachineExecutorMockRecorder {
	return m.recorder
}

// Execute mocks base method.
func (m *MockStateMachineExecutor) Execute(stateMachineARN string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", stateMachineARN)
	ret0, _ := ret[0].(error)
	return ret0
}

// Execute indicates an expected call of Execute.
func (mr *MockStateMachineExecutorMockRecorder) Execute(stateMachineARN interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockStateMachineExecutor)(nil).Execute), stateMachineARN)
}

// MockCFNStackResourceLister is a mock of CFNStackResourceLister interface.
type MockCFNStackResourceLister struct {
	ctrl     *gomock.Controller
	recorder *MockCFNStackResourceListerMockRecorder
}

// MockCFNStackResourceListerMockRecorder is the mock recorder for MockCFNStackResourceLister.
type MockCFNStackResourceListerMockRecorder struct {
	mock *MockCFNStackResourceLister
}

// NewMockCFNStackResourceLister creates a new mock instance.
func NewMockCFNStackResourceLister(ctrl *gomock.Controller) *MockCFNStackResourceLister {
	mock := &MockCFNStackResourceLister{ctrl: ctrl}
	mock.recorder = &MockCFNStackResourceListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCFNStackResourceLister) EXPECT() *MockCFNStackResourceListerMockRecorder {
	return m.recorder
}

// StackResources mocks base method.
func (m *MockCFNStackResourceLister) StackResources(name string) ([]*cloudformation.StackResource, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StackResources", name)
	ret0, _ := ret[0].([]*cloudformation.StackResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StackResources indicates an expected call of StackResources.
func (mr *MockCFNStackResourceListerMockRecorder) StackResources(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StackResources", reflect.TypeOf((*MockCFNStackResourceLister)(nil).StackResources), name)
}
