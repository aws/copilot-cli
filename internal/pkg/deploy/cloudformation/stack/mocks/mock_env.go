// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/deploy/cloudformation/stack/env.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	template "github.com/aws/copilot-cli/internal/pkg/template"
	gomock "github.com/golang/mock/gomock"
)

// MockenvReadParser is a mock of envReadParser interface.
type MockenvReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockenvReadParserMockRecorder
}

// MockenvReadParserMockRecorder is the mock recorder for MockenvReadParser.
type MockenvReadParserMockRecorder struct {
	mock *MockenvReadParser
}

// NewMockenvReadParser creates a new mock instance.
func NewMockenvReadParser(ctrl *gomock.Controller) *MockenvReadParser {
	mock := &MockenvReadParser{ctrl: ctrl}
	mock.recorder = &MockenvReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockenvReadParser) EXPECT() *MockenvReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockenvReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{path, data}
	for _, a := range options {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Parse", varargs...)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parse indicates an expected call of Parse.
func (mr *MockenvReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockenvReadParser)(nil).Parse), varargs...)
}

// ParseEnv mocks base method.
func (m *MockenvReadParser) ParseEnv(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{data}
	for _, a := range options {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ParseEnv", varargs...)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseEnv indicates an expected call of ParseEnv.
func (mr *MockenvReadParserMockRecorder) ParseEnv(data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseEnv", reflect.TypeOf((*MockenvReadParser)(nil).ParseEnv), varargs...)
}

// ParseEnvBootstrap mocks base method.
func (m *MockenvReadParser) ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{data}
	for _, a := range options {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ParseEnvBootstrap", varargs...)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseEnvBootstrap indicates an expected call of ParseEnvBootstrap.
func (mr *MockenvReadParserMockRecorder) ParseEnvBootstrap(data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseEnvBootstrap", reflect.TypeOf((*MockenvReadParser)(nil).ParseEnvBootstrap), varargs...)
}

// Read mocks base method.
func (m *MockenvReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockenvReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockenvReadParser)(nil).Read), path)
}
