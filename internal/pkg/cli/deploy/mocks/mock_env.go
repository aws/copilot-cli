// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/cli/deploy/env.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	cloudformation "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	config "github.com/aws/copilot-cli/internal/pkg/config"
	deploy "github.com/aws/copilot-cli/internal/pkg/deploy"
	stack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	progress "github.com/aws/copilot-cli/internal/pkg/term/progress"
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

// MockenvironmentDeployer is a mock of environmentDeployer interface.
type MockenvironmentDeployer struct {
	ctrl     *gomock.Controller
	recorder *MockenvironmentDeployerMockRecorder
}

// MockenvironmentDeployerMockRecorder is the mock recorder for MockenvironmentDeployer.
type MockenvironmentDeployerMockRecorder struct {
	mock *MockenvironmentDeployer
}

// NewMockenvironmentDeployer creates a new mock instance.
func NewMockenvironmentDeployer(ctrl *gomock.Controller) *MockenvironmentDeployer {
	mock := &MockenvironmentDeployer{ctrl: ctrl}
	mock.recorder = &MockenvironmentDeployerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockenvironmentDeployer) EXPECT() *MockenvironmentDeployerMockRecorder {
	return m.recorder
}

// UpdateAndRenderEnvironment mocks base method.
func (m *MockenvironmentDeployer) UpdateAndRenderEnvironment(out progress.FileWriter, env *deploy.CreateEnvironmentInput, opts ...cloudformation.StackOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{out, env}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateAndRenderEnvironment", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateAndRenderEnvironment indicates an expected call of UpdateAndRenderEnvironment.
func (mr *MockenvironmentDeployerMockRecorder) UpdateAndRenderEnvironment(out, env interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{out, env}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAndRenderEnvironment", reflect.TypeOf((*MockenvironmentDeployer)(nil).UpdateAndRenderEnvironment), varargs...)
}

// MockprefixListGetter is a mock of prefixListGetter interface.
type MockprefixListGetter struct {
	ctrl     *gomock.Controller
	recorder *MockprefixListGetterMockRecorder
}

// MockprefixListGetterMockRecorder is the mock recorder for MockprefixListGetter.
type MockprefixListGetterMockRecorder struct {
	mock *MockprefixListGetter
}

// NewMockprefixListGetter creates a new mock instance.
func NewMockprefixListGetter(ctrl *gomock.Controller) *MockprefixListGetter {
	mock := &MockprefixListGetter{ctrl: ctrl}
	mock.recorder = &MockprefixListGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockprefixListGetter) EXPECT() *MockprefixListGetterMockRecorder {
	return m.recorder
}

// CloudFrontManagedPrefixListID mocks base method.
func (m *MockprefixListGetter) CloudFrontManagedPrefixListID() (*string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudFrontManagedPrefixListID")
	ret0, _ := ret[0].(*string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CloudFrontManagedPrefixListID indicates an expected call of CloudFrontManagedPrefixListID.
func (mr *MockprefixListGetterMockRecorder) CloudFrontManagedPrefixListID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudFrontManagedPrefixListID", reflect.TypeOf((*MockprefixListGetter)(nil).CloudFrontManagedPrefixListID))
}
