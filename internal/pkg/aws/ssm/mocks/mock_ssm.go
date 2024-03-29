// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/aws/ssm/ssm.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	request "github.com/aws/aws-sdk-go/aws/request"
	ssm "github.com/aws/aws-sdk-go/service/ssm"
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

// AddTagsToResource mocks base method.
func (m *Mockapi) AddTagsToResource(arg0 *ssm.AddTagsToResourceInput) (*ssm.AddTagsToResourceOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTagsToResource", arg0)
	ret0, _ := ret[0].(*ssm.AddTagsToResourceOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddTagsToResource indicates an expected call of AddTagsToResource.
func (mr *MockapiMockRecorder) AddTagsToResource(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTagsToResource", reflect.TypeOf((*Mockapi)(nil).AddTagsToResource), arg0)
}

// GetParameterWithContext mocks base method.
func (m *Mockapi) GetParameterWithContext(arg0 context.Context, arg1 *ssm.GetParameterInput, arg2 ...request.Option) (*ssm.GetParameterOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetParameterWithContext", varargs...)
	ret0, _ := ret[0].(*ssm.GetParameterOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetParameterWithContext indicates an expected call of GetParameterWithContext.
func (mr *MockapiMockRecorder) GetParameterWithContext(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetParameterWithContext", reflect.TypeOf((*Mockapi)(nil).GetParameterWithContext), varargs...)
}

// PutParameter mocks base method.
func (m *Mockapi) PutParameter(arg0 *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutParameter", arg0)
	ret0, _ := ret[0].(*ssm.PutParameterOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutParameter indicates an expected call of PutParameter.
func (mr *MockapiMockRecorder) PutParameter(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutParameter", reflect.TypeOf((*Mockapi)(nil).PutParameter), arg0)
}
