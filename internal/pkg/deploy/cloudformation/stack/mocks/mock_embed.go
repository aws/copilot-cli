// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/deploy/cloudformation/stack/embed.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	template "github.com/aws/copilot-cli/internal/pkg/template"
	gomock "github.com/golang/mock/gomock"
)

// MockloadBalancedWebSvcReadParser is a mock of loadBalancedWebSvcReadParser interface.
type MockloadBalancedWebSvcReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockloadBalancedWebSvcReadParserMockRecorder
}

// MockloadBalancedWebSvcReadParserMockRecorder is the mock recorder for MockloadBalancedWebSvcReadParser.
type MockloadBalancedWebSvcReadParserMockRecorder struct {
	mock *MockloadBalancedWebSvcReadParser
}

// NewMockloadBalancedWebSvcReadParser creates a new mock instance.
func NewMockloadBalancedWebSvcReadParser(ctrl *gomock.Controller) *MockloadBalancedWebSvcReadParser {
	mock := &MockloadBalancedWebSvcReadParser{ctrl: ctrl}
	mock.recorder = &MockloadBalancedWebSvcReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockloadBalancedWebSvcReadParser) EXPECT() *MockloadBalancedWebSvcReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockloadBalancedWebSvcReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockloadBalancedWebSvcReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockloadBalancedWebSvcReadParser)(nil).Parse), varargs...)
}

// ParseLoadBalancedWebService mocks base method.
func (m *MockloadBalancedWebSvcReadParser) ParseLoadBalancedWebService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseLoadBalancedWebService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseLoadBalancedWebService indicates an expected call of ParseLoadBalancedWebService.
func (mr *MockloadBalancedWebSvcReadParserMockRecorder) ParseLoadBalancedWebService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseLoadBalancedWebService", reflect.TypeOf((*MockloadBalancedWebSvcReadParser)(nil).ParseLoadBalancedWebService), arg0)
}

// Read mocks base method.
func (m *MockloadBalancedWebSvcReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockloadBalancedWebSvcReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockloadBalancedWebSvcReadParser)(nil).Read), path)
}

// MockbackendSvcReadParser is a mock of backendSvcReadParser interface.
type MockbackendSvcReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockbackendSvcReadParserMockRecorder
}

// MockbackendSvcReadParserMockRecorder is the mock recorder for MockbackendSvcReadParser.
type MockbackendSvcReadParserMockRecorder struct {
	mock *MockbackendSvcReadParser
}

// NewMockbackendSvcReadParser creates a new mock instance.
func NewMockbackendSvcReadParser(ctrl *gomock.Controller) *MockbackendSvcReadParser {
	mock := &MockbackendSvcReadParser{ctrl: ctrl}
	mock.recorder = &MockbackendSvcReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockbackendSvcReadParser) EXPECT() *MockbackendSvcReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockbackendSvcReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockbackendSvcReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockbackendSvcReadParser)(nil).Parse), varargs...)
}

// ParseBackendService mocks base method.
func (m *MockbackendSvcReadParser) ParseBackendService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseBackendService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseBackendService indicates an expected call of ParseBackendService.
func (mr *MockbackendSvcReadParserMockRecorder) ParseBackendService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseBackendService", reflect.TypeOf((*MockbackendSvcReadParser)(nil).ParseBackendService), arg0)
}

// Read mocks base method.
func (m *MockbackendSvcReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockbackendSvcReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockbackendSvcReadParser)(nil).Read), path)
}

// MockrequestDrivenWebSvcReadParser is a mock of requestDrivenWebSvcReadParser interface.
type MockrequestDrivenWebSvcReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockrequestDrivenWebSvcReadParserMockRecorder
}

// MockrequestDrivenWebSvcReadParserMockRecorder is the mock recorder for MockrequestDrivenWebSvcReadParser.
type MockrequestDrivenWebSvcReadParserMockRecorder struct {
	mock *MockrequestDrivenWebSvcReadParser
}

// NewMockrequestDrivenWebSvcReadParser creates a new mock instance.
func NewMockrequestDrivenWebSvcReadParser(ctrl *gomock.Controller) *MockrequestDrivenWebSvcReadParser {
	mock := &MockrequestDrivenWebSvcReadParser{ctrl: ctrl}
	mock.recorder = &MockrequestDrivenWebSvcReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockrequestDrivenWebSvcReadParser) EXPECT() *MockrequestDrivenWebSvcReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockrequestDrivenWebSvcReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockrequestDrivenWebSvcReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockrequestDrivenWebSvcReadParser)(nil).Parse), varargs...)
}

// ParseRequestDrivenWebService mocks base method.
func (m *MockrequestDrivenWebSvcReadParser) ParseRequestDrivenWebService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseRequestDrivenWebService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseRequestDrivenWebService indicates an expected call of ParseRequestDrivenWebService.
func (mr *MockrequestDrivenWebSvcReadParserMockRecorder) ParseRequestDrivenWebService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseRequestDrivenWebService", reflect.TypeOf((*MockrequestDrivenWebSvcReadParser)(nil).ParseRequestDrivenWebService), arg0)
}

// Read mocks base method.
func (m *MockrequestDrivenWebSvcReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockrequestDrivenWebSvcReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockrequestDrivenWebSvcReadParser)(nil).Read), path)
}

// MockworkerSvcReadParser is a mock of workerSvcReadParser interface.
type MockworkerSvcReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockworkerSvcReadParserMockRecorder
}

// MockworkerSvcReadParserMockRecorder is the mock recorder for MockworkerSvcReadParser.
type MockworkerSvcReadParserMockRecorder struct {
	mock *MockworkerSvcReadParser
}

// NewMockworkerSvcReadParser creates a new mock instance.
func NewMockworkerSvcReadParser(ctrl *gomock.Controller) *MockworkerSvcReadParser {
	mock := &MockworkerSvcReadParser{ctrl: ctrl}
	mock.recorder = &MockworkerSvcReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockworkerSvcReadParser) EXPECT() *MockworkerSvcReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockworkerSvcReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockworkerSvcReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockworkerSvcReadParser)(nil).Parse), varargs...)
}

// ParseWorkerService mocks base method.
func (m *MockworkerSvcReadParser) ParseWorkerService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseWorkerService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseWorkerService indicates an expected call of ParseWorkerService.
func (mr *MockworkerSvcReadParserMockRecorder) ParseWorkerService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseWorkerService", reflect.TypeOf((*MockworkerSvcReadParser)(nil).ParseWorkerService), arg0)
}

// Read mocks base method.
func (m *MockworkerSvcReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockworkerSvcReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockworkerSvcReadParser)(nil).Read), path)
}

// MockscheduledJobReadParser is a mock of scheduledJobReadParser interface.
type MockscheduledJobReadParser struct {
	ctrl     *gomock.Controller
	recorder *MockscheduledJobReadParserMockRecorder
}

// MockscheduledJobReadParserMockRecorder is the mock recorder for MockscheduledJobReadParser.
type MockscheduledJobReadParserMockRecorder struct {
	mock *MockscheduledJobReadParser
}

// NewMockscheduledJobReadParser creates a new mock instance.
func NewMockscheduledJobReadParser(ctrl *gomock.Controller) *MockscheduledJobReadParser {
	mock := &MockscheduledJobReadParser{ctrl: ctrl}
	mock.recorder = &MockscheduledJobReadParserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockscheduledJobReadParser) EXPECT() *MockscheduledJobReadParserMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockscheduledJobReadParser) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockscheduledJobReadParserMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockscheduledJobReadParser)(nil).Parse), varargs...)
}

// ParseScheduledJob mocks base method.
func (m *MockscheduledJobReadParser) ParseScheduledJob(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseScheduledJob", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseScheduledJob indicates an expected call of ParseScheduledJob.
func (mr *MockscheduledJobReadParserMockRecorder) ParseScheduledJob(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseScheduledJob", reflect.TypeOf((*MockscheduledJobReadParser)(nil).ParseScheduledJob), arg0)
}

// Read mocks base method.
func (m *MockscheduledJobReadParser) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockscheduledJobReadParserMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockscheduledJobReadParser)(nil).Read), path)
}

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
func (m *MockenvReadParser) ParseEnv(data *template.EnvOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseEnv", data)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseEnv indicates an expected call of ParseEnv.
func (mr *MockenvReadParserMockRecorder) ParseEnv(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseEnv", reflect.TypeOf((*MockenvReadParser)(nil).ParseEnv), data)
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

// MockembedFS is a mock of embedFS interface.
type MockembedFS struct {
	ctrl     *gomock.Controller
	recorder *MockembedFSMockRecorder
}

// MockembedFSMockRecorder is the mock recorder for MockembedFS.
type MockembedFSMockRecorder struct {
	mock *MockembedFS
}

// NewMockembedFS creates a new mock instance.
func NewMockembedFS(ctrl *gomock.Controller) *MockembedFS {
	mock := &MockembedFS{ctrl: ctrl}
	mock.recorder = &MockembedFSMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockembedFS) EXPECT() *MockembedFSMockRecorder {
	return m.recorder
}

// Parse mocks base method.
func (m *MockembedFS) Parse(path string, data interface{}, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockembedFSMockRecorder) Parse(path, data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{path, data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockembedFS)(nil).Parse), varargs...)
}

// ParseBackendService mocks base method.
func (m *MockembedFS) ParseBackendService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseBackendService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseBackendService indicates an expected call of ParseBackendService.
func (mr *MockembedFSMockRecorder) ParseBackendService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseBackendService", reflect.TypeOf((*MockembedFS)(nil).ParseBackendService), arg0)
}

// ParseEnv mocks base method.
func (m *MockembedFS) ParseEnv(data *template.EnvOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseEnv", data)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseEnv indicates an expected call of ParseEnv.
func (mr *MockembedFSMockRecorder) ParseEnv(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseEnv", reflect.TypeOf((*MockembedFS)(nil).ParseEnv), data)
}

// ParseEnvBootstrap mocks base method.
func (m *MockembedFS) ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error) {
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
func (mr *MockembedFSMockRecorder) ParseEnvBootstrap(data interface{}, options ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{data}, options...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseEnvBootstrap", reflect.TypeOf((*MockembedFS)(nil).ParseEnvBootstrap), varargs...)
}

// ParseLoadBalancedWebService mocks base method.
func (m *MockembedFS) ParseLoadBalancedWebService(arg0 template.WorkloadOpts) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ParseLoadBalancedWebService", arg0)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ParseLoadBalancedWebService indicates an expected call of ParseLoadBalancedWebService.
func (mr *MockembedFSMockRecorder) ParseLoadBalancedWebService(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ParseLoadBalancedWebService", reflect.TypeOf((*MockembedFS)(nil).ParseLoadBalancedWebService), arg0)
}

// Read mocks base method.
func (m *MockembedFS) Read(path string) (*template.Content, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", path)
	ret0, _ := ret[0].(*template.Content)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockembedFSMockRecorder) Read(path interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockembedFS)(nil).Read), path)
}
