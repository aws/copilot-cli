// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/cli/deploy/env.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	config "github.com/aws/copilot-cli/internal/pkg/config"
	stack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	gomock "github.com/golang/mock/gomock"
)

// MockappResourcesGetter is a mock of appResourcesGetter interface.
type MockappResourcesGetter struct {
	ctrl     *gomock.Controller
	recorder *MockappResourcesGetterMockRecorder
}

// MockappResourcesGetterMockRecorder is the mock recorder for MockappResourcesGetter.
type MockappResourcesGetterMockRecorder struct {
	mock *MockappResourcesGetter
}

// NewMockappResourcesGetter creates a new mock instance.
func NewMockappResourcesGetter(ctrl *gomock.Controller) *MockappResourcesGetter {
	mock := &MockappResourcesGetter{ctrl: ctrl}
	mock.recorder = &MockappResourcesGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockappResourcesGetter) EXPECT() *MockappResourcesGetterMockRecorder {
	return m.recorder
}

// GetAppResourcesByRegion mocks base method.
func (m *MockappResourcesGetter) GetAppResourcesByRegion(app *config.Application, region string) (*stack.AppRegionalResources, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAppResourcesByRegion", app, region)
	ret0, _ := ret[0].(*stack.AppRegionalResources)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAppResourcesByRegion indicates an expected call of GetAppResourcesByRegion.
func (mr *MockappResourcesGetterMockRecorder) GetAppResourcesByRegion(app, region interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAppResourcesByRegion", reflect.TypeOf((*MockappResourcesGetter)(nil).GetAppResourcesByRegion), app, region)
}

// GetRegionalAppResources mocks base method.
func (m *MockappResourcesGetter) GetRegionalAppResources(app *config.Application) ([]*stack.AppRegionalResources, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRegionalAppResources", app)
	ret0, _ := ret[0].([]*stack.AppRegionalResources)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRegionalAppResources indicates an expected call of GetRegionalAppResources.
func (mr *MockappResourcesGetterMockRecorder) GetRegionalAppResources(app interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRegionalAppResources", reflect.TypeOf((*MockappResourcesGetter)(nil).GetRegionalAppResources), app)
}
