// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/aws/apprunner/apprunner.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	aws "github.com/aws/aws-sdk-go/aws"
	request "github.com/aws/aws-sdk-go/aws/request"
	apprunner "github.com/aws/aws-sdk-go/service/apprunner"
	gomock "github.com/golang/mock/gomock"
)

// Mockapi is a mock of api interface.
type Mockapi struct {
	ctrl     *gomock.Controller
	recorder *MockapiMockRecorder
}

// MockapiMockRecorder is the mock recorder for Mockapi.
type MockapiMockRecorder struct {
	mock *Mockapi
}

// NewMockapi creates a new mock instance.
func NewMockapi(ctrl *gomock.Controller) *Mockapi {
	mock := &Mockapi{ctrl: ctrl}
	mock.recorder = &MockapiMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockapi) EXPECT() *MockapiMockRecorder {
	return m.recorder
}

// DescribeObservabilityConfiguration mocks base method.
func (m *Mockapi) DescribeObservabilityConfiguration(input *apprunner.DescribeObservabilityConfigurationInput) (*apprunner.DescribeObservabilityConfigurationOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeObservabilityConfiguration", input)
	ret0, _ := ret[0].(*apprunner.DescribeObservabilityConfigurationOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeObservabilityConfiguration indicates an expected call of DescribeObservabilityConfiguration.
func (mr *MockapiMockRecorder) DescribeObservabilityConfiguration(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeObservabilityConfiguration", reflect.TypeOf((*Mockapi)(nil).DescribeObservabilityConfiguration), input)
}

// DescribeService mocks base method.
func (m *Mockapi) DescribeService(input *apprunner.DescribeServiceInput) (*apprunner.DescribeServiceOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeService", input)
	ret0, _ := ret[0].(*apprunner.DescribeServiceOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeService indicates an expected call of DescribeService.
func (mr *MockapiMockRecorder) DescribeService(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeService", reflect.TypeOf((*Mockapi)(nil).DescribeService), input)
}

// DescribeVpcIngressConnectionWithContext mocks base method.
func (m *Mockapi) DescribeVpcIngressConnectionWithContext(ctx aws.Context, input *apprunner.DescribeVpcIngressConnectionInput, opts ...request.Option) (*apprunner.DescribeVpcIngressConnectionOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, input}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "DescribeVpcIngressConnectionWithContext", varargs...)
	ret0, _ := ret[0].(*apprunner.DescribeVpcIngressConnectionOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeVpcIngressConnectionWithContext indicates an expected call of DescribeVpcIngressConnectionWithContext.
func (mr *MockapiMockRecorder) DescribeVpcIngressConnectionWithContext(ctx, input interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, input}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeVpcIngressConnectionWithContext", reflect.TypeOf((*Mockapi)(nil).DescribeVpcIngressConnectionWithContext), varargs...)
}

// ListOperations mocks base method.
func (m *Mockapi) ListOperations(input *apprunner.ListOperationsInput) (*apprunner.ListOperationsOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListOperations", input)
	ret0, _ := ret[0].(*apprunner.ListOperationsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListOperations indicates an expected call of ListOperations.
func (mr *MockapiMockRecorder) ListOperations(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperations", reflect.TypeOf((*Mockapi)(nil).ListOperations), input)
}

// ListServices mocks base method.
func (m *Mockapi) ListServices(input *apprunner.ListServicesInput) (*apprunner.ListServicesOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListServices", input)
	ret0, _ := ret[0].(*apprunner.ListServicesOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListServices indicates an expected call of ListServices.
func (mr *MockapiMockRecorder) ListServices(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListServices", reflect.TypeOf((*Mockapi)(nil).ListServices), input)
}

// PauseService mocks base method.
func (m *Mockapi) PauseService(input *apprunner.PauseServiceInput) (*apprunner.PauseServiceOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PauseService", input)
	ret0, _ := ret[0].(*apprunner.PauseServiceOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PauseService indicates an expected call of PauseService.
func (mr *MockapiMockRecorder) PauseService(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PauseService", reflect.TypeOf((*Mockapi)(nil).PauseService), input)
}

// ResumeService mocks base method.
func (m *Mockapi) ResumeService(input *apprunner.ResumeServiceInput) (*apprunner.ResumeServiceOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResumeService", input)
	ret0, _ := ret[0].(*apprunner.ResumeServiceOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResumeService indicates an expected call of ResumeService.
func (mr *MockapiMockRecorder) ResumeService(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResumeService", reflect.TypeOf((*Mockapi)(nil).ResumeService), input)
}

// StartDeployment mocks base method.
func (m *Mockapi) StartDeployment(input *apprunner.StartDeploymentInput) (*apprunner.StartDeploymentOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartDeployment", input)
	ret0, _ := ret[0].(*apprunner.StartDeploymentOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartDeployment indicates an expected call of StartDeployment.
func (mr *MockapiMockRecorder) StartDeployment(input interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartDeployment", reflect.TypeOf((*Mockapi)(nil).StartDeployment), input)
}
