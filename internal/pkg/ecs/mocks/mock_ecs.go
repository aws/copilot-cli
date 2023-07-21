// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/ecs/ecs.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	ecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	resourcegroups "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
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

// MockecsClient is a mock of ecsClient interface.
type MockecsClient struct {
	ctrl     *gomock.Controller
	recorder *MockecsClientMockRecorder
}

// MockecsClientMockRecorder is the mock recorder for MockecsClient.
type MockecsClientMockRecorder struct {
	mock *MockecsClient
}

// NewMockecsClient creates a new mock instance.
func NewMockecsClient(ctrl *gomock.Controller) *MockecsClient {
	mock := &MockecsClient{ctrl: ctrl}
	mock.recorder = &MockecsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockecsClient) EXPECT() *MockecsClientMockRecorder {
	return m.recorder
}

// ActiveClusters mocks base method.
func (m *MockecsClient) ActiveClusters(arns ...string) ([]string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range arns {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ActiveClusters", varargs...)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ActiveClusters indicates an expected call of ActiveClusters.
func (mr *MockecsClientMockRecorder) ActiveClusters(arns ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActiveClusters", reflect.TypeOf((*MockecsClient)(nil).ActiveClusters), arns...)
}

// DefaultCluster mocks base method.
func (m *MockecsClient) DefaultCluster() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DefaultCluster")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DefaultCluster indicates an expected call of DefaultCluster.
func (mr *MockecsClientMockRecorder) DefaultCluster() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DefaultCluster", reflect.TypeOf((*MockecsClient)(nil).DefaultCluster))
}

// DescribeTasks mocks base method.
func (m *MockecsClient) DescribeTasks(cluster string, taskARNs []string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeTasks", cluster, taskARNs)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeTasks indicates an expected call of DescribeTasks.
func (mr *MockecsClientMockRecorder) DescribeTasks(cluster, taskARNs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeTasks", reflect.TypeOf((*MockecsClient)(nil).DescribeTasks), cluster, taskARNs)
}

// NetworkConfiguration mocks base method.
func (m *MockecsClient) NetworkConfiguration(cluster, serviceName string) (*ecs.NetworkConfiguration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NetworkConfiguration", cluster, serviceName)
	ret0, _ := ret[0].(*ecs.NetworkConfiguration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NetworkConfiguration indicates an expected call of NetworkConfiguration.
func (mr *MockecsClientMockRecorder) NetworkConfiguration(cluster, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NetworkConfiguration", reflect.TypeOf((*MockecsClient)(nil).NetworkConfiguration), cluster, serviceName)
}

// RunningTasks mocks base method.
func (m *MockecsClient) RunningTasks(cluster string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunningTasks", cluster)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunningTasks indicates an expected call of RunningTasks.
func (mr *MockecsClientMockRecorder) RunningTasks(cluster interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunningTasks", reflect.TypeOf((*MockecsClient)(nil).RunningTasks), cluster)
}

// RunningTasksInFamily mocks base method.
func (m *MockecsClient) RunningTasksInFamily(cluster, family string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunningTasksInFamily", cluster, family)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RunningTasksInFamily indicates an expected call of RunningTasksInFamily.
func (mr *MockecsClientMockRecorder) RunningTasksInFamily(cluster, family interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunningTasksInFamily", reflect.TypeOf((*MockecsClient)(nil).RunningTasksInFamily), cluster, family)
}

// Service mocks base method.
func (m *MockecsClient) Service(clusterName, serviceName string) (*ecs.Service, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Service", clusterName, serviceName)
	ret0, _ := ret[0].(*ecs.Service)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Service indicates an expected call of Service.
func (mr *MockecsClientMockRecorder) Service(clusterName, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Service", reflect.TypeOf((*MockecsClient)(nil).Service), clusterName, serviceName)
}

// ServiceRunningTasks mocks base method.
func (m *MockecsClient) ServiceRunningTasks(clusterName, serviceName string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ServiceRunningTasks", clusterName, serviceName)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ServiceRunningTasks indicates an expected call of ServiceRunningTasks.
func (mr *MockecsClientMockRecorder) ServiceRunningTasks(clusterName, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ServiceRunningTasks", reflect.TypeOf((*MockecsClient)(nil).ServiceRunningTasks), clusterName, serviceName)
}

// StopTasks mocks base method.
func (m *MockecsClient) StopTasks(tasks []string, opts ...ecs.StopTasksOpts) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{tasks}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StopTasks", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// StopTasks indicates an expected call of StopTasks.
func (mr *MockecsClientMockRecorder) StopTasks(tasks interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{tasks}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopTasks", reflect.TypeOf((*MockecsClient)(nil).StopTasks), varargs...)
}

// StoppedServiceTasks mocks base method.
func (m *MockecsClient) StoppedServiceTasks(cluster, service string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoppedServiceTasks", cluster, service)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StoppedServiceTasks indicates an expected call of StoppedServiceTasks.
func (mr *MockecsClientMockRecorder) StoppedServiceTasks(cluster, service interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoppedServiceTasks", reflect.TypeOf((*MockecsClient)(nil).StoppedServiceTasks), cluster, service)
}

// TaskDefinition mocks base method.
func (m *MockecsClient) TaskDefinition(taskDefName string) (*ecs.TaskDefinition, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TaskDefinition", taskDefName)
	ret0, _ := ret[0].(*ecs.TaskDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TaskDefinition indicates an expected call of TaskDefinition.
func (mr *MockecsClientMockRecorder) TaskDefinition(taskDefName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TaskDefinition", reflect.TypeOf((*MockecsClient)(nil).TaskDefinition), taskDefName)
}

// UpdateService mocks base method.
func (m *MockecsClient) UpdateService(clusterName, serviceName string, opts ...ecs.UpdateServiceOpts) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{clusterName, serviceName}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateService", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateService indicates an expected call of UpdateService.
func (mr *MockecsClientMockRecorder) UpdateService(clusterName, serviceName interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{clusterName, serviceName}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateService", reflect.TypeOf((*MockecsClient)(nil).UpdateService), varargs...)
}

// MockstepFunctionsClient is a mock of stepFunctionsClient interface.
type MockstepFunctionsClient struct {
	ctrl     *gomock.Controller
	recorder *MockstepFunctionsClientMockRecorder
}

// MockstepFunctionsClientMockRecorder is the mock recorder for MockstepFunctionsClient.
type MockstepFunctionsClientMockRecorder struct {
	mock *MockstepFunctionsClient
}

// NewMockstepFunctionsClient creates a new mock instance.
func NewMockstepFunctionsClient(ctrl *gomock.Controller) *MockstepFunctionsClient {
	mock := &MockstepFunctionsClient{ctrl: ctrl}
	mock.recorder = &MockstepFunctionsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockstepFunctionsClient) EXPECT() *MockstepFunctionsClientMockRecorder {
	return m.recorder
}

// StateMachineDefinition mocks base method.
func (m *MockstepFunctionsClient) StateMachineDefinition(stateMachineARN string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateMachineDefinition", stateMachineARN)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateMachineDefinition indicates an expected call of StateMachineDefinition.
func (mr *MockstepFunctionsClientMockRecorder) StateMachineDefinition(stateMachineARN interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateMachineDefinition", reflect.TypeOf((*MockstepFunctionsClient)(nil).StateMachineDefinition), stateMachineARN)
}

// MocksecretGetter is a mock of secretGetter interface.
type MocksecretGetter struct {
	ctrl     *gomock.Controller
	recorder *MocksecretGetterMockRecorder
}

// MocksecretGetterMockRecorder is the mock recorder for MocksecretGetter.
type MocksecretGetterMockRecorder struct {
	mock *MocksecretGetter
}

// NewMocksecretGetter creates a new mock instance.
func NewMocksecretGetter(ctrl *gomock.Controller) *MocksecretGetter {
	mock := &MocksecretGetter{ctrl: ctrl}
	mock.recorder = &MocksecretGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MocksecretGetter) EXPECT() *MocksecretGetterMockRecorder {
	return m.recorder
}

// GetSecretValue mocks base method.
func (m *MocksecretGetter) GetSecretValue(secretName string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSecretValue", secretName)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSecretValue indicates an expected call of GetSecretValue.
func (mr *MocksecretGetterMockRecorder) GetSecretValue(secretName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSecretValue", reflect.TypeOf((*MocksecretGetter)(nil).GetSecretValue), secretName)
}

// IsServiceARN mocks base method.
func (m *MocksecretGetter) IsServiceARN(secretName string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsServiceARN", secretName)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsServiceARN indicates an expected call of IsServiceARN.
func (mr *MocksecretGetterMockRecorder) IsServiceARN(secretName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsServiceARN", reflect.TypeOf((*MocksecretGetter)(nil).IsServiceARN), secretName)
}
