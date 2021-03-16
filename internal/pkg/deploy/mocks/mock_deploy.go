// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/deploy/deploy.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	resourcegroups "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	config "github.com/aws/copilot-cli/internal/pkg/config"
	gomock "github.com/golang/mock/gomock"
)

// MockresourceGetter is a mock of resourceGetter interface.
type MockresourceGetter struct {
	ctrl     *gomock.Controller
	recorder *MockresourceGetterMockRecorder
}

// MockresourceGetterMockRecorder is the mock recorder for MockresourceGetter.
type MockresourceGetterMockRecorder struct {
	mock *MockresourceGetter
}

// NewMockresourceGetter creates a new mock instance.
func NewMockresourceGetter(ctrl *gomock.Controller) *MockresourceGetter {
	mock := &MockresourceGetter{ctrl: ctrl}
	mock.recorder = &MockresourceGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockresourceGetter) EXPECT() *MockresourceGetterMockRecorder {
	return m.recorder
}

// GetResourcesByTags mocks base method.
func (m *MockresourceGetter) GetResourcesByTags(resourceType string, tags map[string]string) ([]*resourcegroups.Resource, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResourcesByTags", resourceType, tags)
	ret0, _ := ret[0].([]*resourcegroups.Resource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetResourcesByTags indicates an expected call of GetResourcesByTags.
func (mr *MockresourceGetterMockRecorder) GetResourcesByTags(resourceType, tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResourcesByTags", reflect.TypeOf((*MockresourceGetter)(nil).GetResourcesByTags), resourceType, tags)
}

// MockConfigStoreClient is a mock of ConfigStoreClient interface.
type MockConfigStoreClient struct {
	ctrl     *gomock.Controller
	recorder *MockConfigStoreClientMockRecorder
}

// MockConfigStoreClientMockRecorder is the mock recorder for MockConfigStoreClient.
type MockConfigStoreClientMockRecorder struct {
	mock *MockConfigStoreClient
}

// NewMockConfigStoreClient creates a new mock instance.
func NewMockConfigStoreClient(ctrl *gomock.Controller) *MockConfigStoreClient {
	mock := &MockConfigStoreClient{ctrl: ctrl}
	mock.recorder = &MockConfigStoreClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfigStoreClient) EXPECT() *MockConfigStoreClientMockRecorder {
	return m.recorder
}

// GetEnvironment mocks base method.
func (m *MockConfigStoreClient) GetEnvironment(appName, environmentName string) (*config.Environment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEnvironment", appName, environmentName)
	ret0, _ := ret[0].(*config.Environment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEnvironment indicates an expected call of GetEnvironment.
func (mr *MockConfigStoreClientMockRecorder) GetEnvironment(appName, environmentName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEnvironment", reflect.TypeOf((*MockConfigStoreClient)(nil).GetEnvironment), appName, environmentName)
}

// GetService mocks base method.
func (m *MockConfigStoreClient) GetService(appName, svcName string) (*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetService", appName, svcName)
	ret0, _ := ret[0].(*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetService indicates an expected call of GetService.
func (mr *MockConfigStoreClientMockRecorder) GetService(appName, svcName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetService", reflect.TypeOf((*MockConfigStoreClient)(nil).GetService), appName, svcName)
}

// ListEnvironments mocks base method.
func (m *MockConfigStoreClient) ListEnvironments(appName string) ([]*config.Environment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListEnvironments", appName)
	ret0, _ := ret[0].([]*config.Environment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListEnvironments indicates an expected call of ListEnvironments.
func (mr *MockConfigStoreClientMockRecorder) ListEnvironments(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListEnvironments", reflect.TypeOf((*MockConfigStoreClient)(nil).ListEnvironments), appName)
}
