// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/describe/status.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	apprunner "github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	cloudwatch "github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"
	cloudwatchlogs "github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	ecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	elbv2 "github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	ecs0 "github.com/aws/copilot-cli/internal/pkg/ecs"
	gomock "github.com/golang/mock/gomock"
)

// MockalarmStatusGetter is a mock of alarmStatusGetter interface.
type MockalarmStatusGetter struct {
	ctrl     *gomock.Controller
	recorder *MockalarmStatusGetterMockRecorder
}

// MockalarmStatusGetterMockRecorder is the mock recorder for MockalarmStatusGetter.
type MockalarmStatusGetterMockRecorder struct {
	mock *MockalarmStatusGetter
}

// NewMockalarmStatusGetter creates a new mock instance.
func NewMockalarmStatusGetter(ctrl *gomock.Controller) *MockalarmStatusGetter {
	mock := &MockalarmStatusGetter{ctrl: ctrl}
	mock.recorder = &MockalarmStatusGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockalarmStatusGetter) EXPECT() *MockalarmStatusGetterMockRecorder {
	return m.recorder
}

// AlarmStatus mocks base method.
func (m *MockalarmStatusGetter) AlarmStatus(alarms []string) ([]cloudwatch.AlarmStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AlarmStatus", alarms)
	ret0, _ := ret[0].([]cloudwatch.AlarmStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AlarmStatus indicates an expected call of AlarmStatus.
func (mr *MockalarmStatusGetterMockRecorder) AlarmStatus(alarms interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AlarmStatus", reflect.TypeOf((*MockalarmStatusGetter)(nil).AlarmStatus), alarms)
}

// AlarmsWithTags mocks base method.
func (m *MockalarmStatusGetter) AlarmsWithTags(tags map[string]string) ([]cloudwatch.AlarmStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AlarmsWithTags", tags)
	ret0, _ := ret[0].([]cloudwatch.AlarmStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AlarmsWithTags indicates an expected call of AlarmsWithTags.
func (mr *MockalarmStatusGetterMockRecorder) AlarmsWithTags(tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AlarmsWithTags", reflect.TypeOf((*MockalarmStatusGetter)(nil).AlarmsWithTags), tags)
}

// MocklogGetter is a mock of logGetter interface.
type MocklogGetter struct {
	ctrl     *gomock.Controller
	recorder *MocklogGetterMockRecorder
}

// MocklogGetterMockRecorder is the mock recorder for MocklogGetter.
type MocklogGetterMockRecorder struct {
	mock *MocklogGetter
}

// NewMocklogGetter creates a new mock instance.
func NewMocklogGetter(ctrl *gomock.Controller) *MocklogGetter {
	mock := &MocklogGetter{ctrl: ctrl}
	mock.recorder = &MocklogGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MocklogGetter) EXPECT() *MocklogGetterMockRecorder {
	return m.recorder
}

// LogEvents mocks base method.
func (m *MocklogGetter) LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LogEvents", opts)
	ret0, _ := ret[0].(*cloudwatchlogs.LogEventsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LogEvents indicates an expected call of LogEvents.
func (mr *MocklogGetterMockRecorder) LogEvents(opts interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LogEvents", reflect.TypeOf((*MocklogGetter)(nil).LogEvents), opts)
}

// MockecsServiceGetter is a mock of ecsServiceGetter interface.
type MockecsServiceGetter struct {
	ctrl     *gomock.Controller
	recorder *MockecsServiceGetterMockRecorder
}

// MockecsServiceGetterMockRecorder is the mock recorder for MockecsServiceGetter.
type MockecsServiceGetterMockRecorder struct {
	mock *MockecsServiceGetter
}

// NewMockecsServiceGetter creates a new mock instance.
func NewMockecsServiceGetter(ctrl *gomock.Controller) *MockecsServiceGetter {
	mock := &MockecsServiceGetter{ctrl: ctrl}
	mock.recorder = &MockecsServiceGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockecsServiceGetter) EXPECT() *MockecsServiceGetterMockRecorder {
	return m.recorder
}

// Service mocks base method.
func (m *MockecsServiceGetter) Service(clusterName, serviceName string) (*ecs.Service, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Service", clusterName, serviceName)
	ret0, _ := ret[0].(*ecs.Service)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Service indicates an expected call of Service.
func (mr *MockecsServiceGetterMockRecorder) Service(clusterName, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Service", reflect.TypeOf((*MockecsServiceGetter)(nil).Service), clusterName, serviceName)
}

// ServiceTasks mocks base method.
func (m *MockecsServiceGetter) ServiceTasks(clusterName, serviceName string) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ServiceTasks", clusterName, serviceName)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ServiceTasks indicates an expected call of ServiceTasks.
func (mr *MockecsServiceGetterMockRecorder) ServiceTasks(clusterName, serviceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ServiceTasks", reflect.TypeOf((*MockecsServiceGetter)(nil).ServiceTasks), clusterName, serviceName)
}

// MockserviceDescriber is a mock of serviceDescriber interface.
type MockserviceDescriber struct {
	ctrl     *gomock.Controller
	recorder *MockserviceDescriberMockRecorder
}

// MockserviceDescriberMockRecorder is the mock recorder for MockserviceDescriber.
type MockserviceDescriberMockRecorder struct {
	mock *MockserviceDescriber
}

// NewMockserviceDescriber creates a new mock instance.
func NewMockserviceDescriber(ctrl *gomock.Controller) *MockserviceDescriber {
	mock := &MockserviceDescriber{ctrl: ctrl}
	mock.recorder = &MockserviceDescriberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockserviceDescriber) EXPECT() *MockserviceDescriberMockRecorder {
	return m.recorder
}

// DescribeService mocks base method.
func (m *MockserviceDescriber) DescribeService(app, env, svc string) (*ecs0.ServiceDesc, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeService", app, env, svc)
	ret0, _ := ret[0].(*ecs0.ServiceDesc)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeService indicates an expected call of DescribeService.
func (mr *MockserviceDescriberMockRecorder) DescribeService(app, env, svc interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeService", reflect.TypeOf((*MockserviceDescriber)(nil).DescribeService), app, env, svc)
}

// MockapprunnerServiceDescriber is a mock of apprunnerServiceDescriber interface.
type MockapprunnerServiceDescriber struct {
	ctrl     *gomock.Controller
	recorder *MockapprunnerServiceDescriberMockRecorder
}

// MockapprunnerServiceDescriberMockRecorder is the mock recorder for MockapprunnerServiceDescriber.
type MockapprunnerServiceDescriberMockRecorder struct {
	mock *MockapprunnerServiceDescriber
}

// NewMockapprunnerServiceDescriber creates a new mock instance.
func NewMockapprunnerServiceDescriber(ctrl *gomock.Controller) *MockapprunnerServiceDescriber {
	mock := &MockapprunnerServiceDescriber{ctrl: ctrl}
	mock.recorder = &MockapprunnerServiceDescriberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockapprunnerServiceDescriber) EXPECT() *MockapprunnerServiceDescriberMockRecorder {
	return m.recorder
}

// Service mocks base method.
func (m *MockapprunnerServiceDescriber) Service() (*apprunner.Service, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Service")
	ret0, _ := ret[0].(*apprunner.Service)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Service indicates an expected call of Service.
func (mr *MockapprunnerServiceDescriberMockRecorder) Service() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Service", reflect.TypeOf((*MockapprunnerServiceDescriber)(nil).Service))
}

// MockautoscalingAlarmNamesGetter is a mock of autoscalingAlarmNamesGetter interface.
type MockautoscalingAlarmNamesGetter struct {
	ctrl     *gomock.Controller
	recorder *MockautoscalingAlarmNamesGetterMockRecorder
}

// MockautoscalingAlarmNamesGetterMockRecorder is the mock recorder for MockautoscalingAlarmNamesGetter.
type MockautoscalingAlarmNamesGetterMockRecorder struct {
	mock *MockautoscalingAlarmNamesGetter
}

// NewMockautoscalingAlarmNamesGetter creates a new mock instance.
func NewMockautoscalingAlarmNamesGetter(ctrl *gomock.Controller) *MockautoscalingAlarmNamesGetter {
	mock := &MockautoscalingAlarmNamesGetter{ctrl: ctrl}
	mock.recorder = &MockautoscalingAlarmNamesGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockautoscalingAlarmNamesGetter) EXPECT() *MockautoscalingAlarmNamesGetterMockRecorder {
	return m.recorder
}

// ECSServiceAlarmNames mocks base method.
func (m *MockautoscalingAlarmNamesGetter) ECSServiceAlarmNames(cluster, service string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ECSServiceAlarmNames", cluster, service)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ECSServiceAlarmNames indicates an expected call of ECSServiceAlarmNames.
func (mr *MockautoscalingAlarmNamesGetterMockRecorder) ECSServiceAlarmNames(cluster, service interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ECSServiceAlarmNames", reflect.TypeOf((*MockautoscalingAlarmNamesGetter)(nil).ECSServiceAlarmNames), cluster, service)
}

// MocktargetHealthGetter is a mock of targetHealthGetter interface.
type MocktargetHealthGetter struct {
	ctrl     *gomock.Controller
	recorder *MocktargetHealthGetterMockRecorder
}

// MocktargetHealthGetterMockRecorder is the mock recorder for MocktargetHealthGetter.
type MocktargetHealthGetterMockRecorder struct {
	mock *MocktargetHealthGetter
}

// NewMocktargetHealthGetter creates a new mock instance.
func NewMocktargetHealthGetter(ctrl *gomock.Controller) *MocktargetHealthGetter {
	mock := &MocktargetHealthGetter{ctrl: ctrl}
	mock.recorder = &MocktargetHealthGetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MocktargetHealthGetter) EXPECT() *MocktargetHealthGetterMockRecorder {
	return m.recorder
}

// TargetsHealth mocks base method.
func (m *MocktargetHealthGetter) TargetsHealth(targetGroupARN string) ([]*elbv2.TargetHealth, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetsHealth", targetGroupARN)
	ret0, _ := ret[0].([]*elbv2.TargetHealth)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TargetsHealth indicates an expected call of TargetsHealth.
func (mr *MocktargetHealthGetterMockRecorder) TargetsHealth(targetGroupARN interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetsHealth", reflect.TypeOf((*MocktargetHealthGetter)(nil).TargetsHealth), targetGroupARN)
}
