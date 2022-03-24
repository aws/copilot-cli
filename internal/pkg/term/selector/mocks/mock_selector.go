// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/term/selector/selector.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	ecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	config "github.com/aws/copilot-cli/internal/pkg/config"
	deploy "github.com/aws/copilot-cli/internal/pkg/deploy"
	ecs0 "github.com/aws/copilot-cli/internal/pkg/ecs"
	prompt "github.com/aws/copilot-cli/internal/pkg/term/prompt"
	workspace "github.com/aws/copilot-cli/internal/pkg/workspace"
	gomock "github.com/golang/mock/gomock"
)

// MockPrompter is a mock of Prompter interface.
type MockPrompter struct {
	ctrl     *gomock.Controller
	recorder *MockPrompterMockRecorder
}

// MockPrompterMockRecorder is the mock recorder for MockPrompter.
type MockPrompterMockRecorder struct {
	mock *MockPrompter
}

// NewMockPrompter creates a new mock instance.
func NewMockPrompter(ctrl *gomock.Controller) *MockPrompter {
	mock := &MockPrompter{ctrl: ctrl}
	mock.recorder = &MockPrompterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPrompter) EXPECT() *MockPrompterMockRecorder {
	return m.recorder
}

// Confirm mocks base method.
func (m *MockPrompter) Confirm(message, help string, promptOpts ...prompt.PromptConfig) (bool, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{message, help}
	for _, a := range promptOpts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Confirm", varargs...)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Confirm indicates an expected call of Confirm.
func (mr *MockPrompterMockRecorder) Confirm(message, help interface{}, promptOpts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{message, help}, promptOpts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Confirm", reflect.TypeOf((*MockPrompter)(nil).Confirm), varargs...)
}

// Get mocks base method.
func (m *MockPrompter) Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{message, help, validator}
	for _, a := range promptOpts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Get", varargs...)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockPrompterMockRecorder) Get(message, help, validator interface{}, promptOpts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{message, help, validator}, promptOpts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPrompter)(nil).Get), varargs...)
}

// MultiSelect mocks base method.
func (m *MockPrompter) MultiSelect(message, help string, options []string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) ([]string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{message, help, options, validator}
	for _, a := range promptOpts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "MultiSelect", varargs...)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MultiSelect indicates an expected call of MultiSelect.
func (mr *MockPrompterMockRecorder) MultiSelect(message, help, options, validator interface{}, promptOpts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{message, help, options, validator}, promptOpts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MultiSelect", reflect.TypeOf((*MockPrompter)(nil).MultiSelect), varargs...)
}

// SelectOne mocks base method.
func (m *MockPrompter) SelectOne(message, help string, options []string, promptOpts ...prompt.PromptConfig) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{message, help, options}
	for _, a := range promptOpts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SelectOne", varargs...)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SelectOne indicates an expected call of SelectOne.
func (mr *MockPrompterMockRecorder) SelectOne(message, help, options interface{}, promptOpts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{message, help, options}, promptOpts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SelectOne", reflect.TypeOf((*MockPrompter)(nil).SelectOne), varargs...)
}

// SelectOption mocks base method.
func (m *MockPrompter) SelectOption(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) (string, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{message, help, opts}
	for _, a := range promptCfgs {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SelectOption", varargs...)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SelectOption indicates an expected call of SelectOption.
func (mr *MockPrompterMockRecorder) SelectOption(message, help, opts interface{}, promptCfgs ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{message, help, opts}, promptCfgs...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SelectOption", reflect.TypeOf((*MockPrompter)(nil).SelectOption), varargs...)
}

// MockAppEnvLister is a mock of AppEnvLister interface.
type MockAppEnvLister struct {
	ctrl     *gomock.Controller
	recorder *MockAppEnvListerMockRecorder
}

// MockAppEnvListerMockRecorder is the mock recorder for MockAppEnvLister.
type MockAppEnvListerMockRecorder struct {
	mock *MockAppEnvLister
}

// NewMockAppEnvLister creates a new mock instance.
func NewMockAppEnvLister(ctrl *gomock.Controller) *MockAppEnvLister {
	mock := &MockAppEnvLister{ctrl: ctrl}
	mock.recorder = &MockAppEnvListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAppEnvLister) EXPECT() *MockAppEnvListerMockRecorder {
	return m.recorder
}

// ListApplications mocks base method.
func (m *MockAppEnvLister) ListApplications() ([]*config.Application, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListApplications")
	ret0, _ := ret[0].([]*config.Application)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListApplications indicates an expected call of ListApplications.
func (mr *MockAppEnvListerMockRecorder) ListApplications() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListApplications", reflect.TypeOf((*MockAppEnvLister)(nil).ListApplications))
}

// ListEnvironments mocks base method.
func (m *MockAppEnvLister) ListEnvironments(appName string) ([]*config.Environment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListEnvironments", appName)
	ret0, _ := ret[0].([]*config.Environment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListEnvironments indicates an expected call of ListEnvironments.
func (mr *MockAppEnvListerMockRecorder) ListEnvironments(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListEnvironments", reflect.TypeOf((*MockAppEnvLister)(nil).ListEnvironments), appName)
}

// MockConfigWorkloadLister is a mock of ConfigWorkloadLister interface.
type MockConfigWorkloadLister struct {
	ctrl     *gomock.Controller
	recorder *MockConfigWorkloadListerMockRecorder
}

// MockConfigWorkloadListerMockRecorder is the mock recorder for MockConfigWorkloadLister.
type MockConfigWorkloadListerMockRecorder struct {
	mock *MockConfigWorkloadLister
}

// NewMockConfigWorkloadLister creates a new mock instance.
func NewMockConfigWorkloadLister(ctrl *gomock.Controller) *MockConfigWorkloadLister {
	mock := &MockConfigWorkloadLister{ctrl: ctrl}
	mock.recorder = &MockConfigWorkloadListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfigWorkloadLister) EXPECT() *MockConfigWorkloadListerMockRecorder {
	return m.recorder
}

// ListJobs mocks base method.
func (m *MockConfigWorkloadLister) ListJobs(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListJobs", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListJobs indicates an expected call of ListJobs.
func (mr *MockConfigWorkloadListerMockRecorder) ListJobs(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListJobs", reflect.TypeOf((*MockConfigWorkloadLister)(nil).ListJobs), appName)
}

// ListServices mocks base method.
func (m *MockConfigWorkloadLister) ListServices(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListServices", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListServices indicates an expected call of ListServices.
func (mr *MockConfigWorkloadListerMockRecorder) ListServices(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListServices", reflect.TypeOf((*MockConfigWorkloadLister)(nil).ListServices), appName)
}

// ListWorkloads mocks base method.
func (m *MockConfigWorkloadLister) ListWorkloads(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListWorkloads", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListWorkloads indicates an expected call of ListWorkloads.
func (mr *MockConfigWorkloadListerMockRecorder) ListWorkloads(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListWorkloads", reflect.TypeOf((*MockConfigWorkloadLister)(nil).ListWorkloads), appName)
}

// MockConfigLister is a mock of ConfigLister interface.
type MockConfigLister struct {
	ctrl     *gomock.Controller
	recorder *MockConfigListerMockRecorder
}

// MockConfigListerMockRecorder is the mock recorder for MockConfigLister.
type MockConfigListerMockRecorder struct {
	mock *MockConfigLister
}

// NewMockConfigLister creates a new mock instance.
func NewMockConfigLister(ctrl *gomock.Controller) *MockConfigLister {
	mock := &MockConfigLister{ctrl: ctrl}
	mock.recorder = &MockConfigListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfigLister) EXPECT() *MockConfigListerMockRecorder {
	return m.recorder
}

// ListApplications mocks base method.
func (m *MockConfigLister) ListApplications() ([]*config.Application, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListApplications")
	ret0, _ := ret[0].([]*config.Application)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListApplications indicates an expected call of ListApplications.
func (mr *MockConfigListerMockRecorder) ListApplications() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListApplications", reflect.TypeOf((*MockConfigLister)(nil).ListApplications))
}

// ListEnvironments mocks base method.
func (m *MockConfigLister) ListEnvironments(appName string) ([]*config.Environment, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListEnvironments", appName)
	ret0, _ := ret[0].([]*config.Environment)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListEnvironments indicates an expected call of ListEnvironments.
func (mr *MockConfigListerMockRecorder) ListEnvironments(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListEnvironments", reflect.TypeOf((*MockConfigLister)(nil).ListEnvironments), appName)
}

// ListJobs mocks base method.
func (m *MockConfigLister) ListJobs(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListJobs", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListJobs indicates an expected call of ListJobs.
func (mr *MockConfigListerMockRecorder) ListJobs(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListJobs", reflect.TypeOf((*MockConfigLister)(nil).ListJobs), appName)
}

// ListServices mocks base method.
func (m *MockConfigLister) ListServices(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListServices", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListServices indicates an expected call of ListServices.
func (mr *MockConfigListerMockRecorder) ListServices(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListServices", reflect.TypeOf((*MockConfigLister)(nil).ListServices), appName)
}

// ListWorkloads mocks base method.
func (m *MockConfigLister) ListWorkloads(appName string) ([]*config.Workload, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListWorkloads", appName)
	ret0, _ := ret[0].([]*config.Workload)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListWorkloads indicates an expected call of ListWorkloads.
func (mr *MockConfigListerMockRecorder) ListWorkloads(appName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListWorkloads", reflect.TypeOf((*MockConfigLister)(nil).ListWorkloads), appName)
}

// MockWsWorkloadLister is a mock of WsWorkloadLister interface.
type MockWsWorkloadLister struct {
	ctrl     *gomock.Controller
	recorder *MockWsWorkloadListerMockRecorder
}

// MockWsWorkloadListerMockRecorder is the mock recorder for MockWsWorkloadLister.
type MockWsWorkloadListerMockRecorder struct {
	mock *MockWsWorkloadLister
}

// NewMockWsWorkloadLister creates a new mock instance.
func NewMockWsWorkloadLister(ctrl *gomock.Controller) *MockWsWorkloadLister {
	mock := &MockWsWorkloadLister{ctrl: ctrl}
	mock.recorder = &MockWsWorkloadListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWsWorkloadLister) EXPECT() *MockWsWorkloadListerMockRecorder {
	return m.recorder
}

// ListJobs mocks base method.
func (m *MockWsWorkloadLister) ListJobs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListJobs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListJobs indicates an expected call of ListJobs.
func (mr *MockWsWorkloadListerMockRecorder) ListJobs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListJobs", reflect.TypeOf((*MockWsWorkloadLister)(nil).ListJobs))
}

// ListServices mocks base method.
func (m *MockWsWorkloadLister) ListServices() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListServices")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListServices indicates an expected call of ListServices.
func (mr *MockWsWorkloadListerMockRecorder) ListServices() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListServices", reflect.TypeOf((*MockWsWorkloadLister)(nil).ListServices))
}

// ListWorkloads mocks base method.
func (m *MockWsWorkloadLister) ListWorkloads() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListWorkloads")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListWorkloads indicates an expected call of ListWorkloads.
func (mr *MockWsWorkloadListerMockRecorder) ListWorkloads() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListWorkloads", reflect.TypeOf((*MockWsWorkloadLister)(nil).ListWorkloads))
}

// MockWsPipelinesLister is a mock of WsPipelinesLister interface.
type MockWsPipelinesLister struct {
	ctrl     *gomock.Controller
	recorder *MockWsPipelinesListerMockRecorder
}

// MockWsPipelinesListerMockRecorder is the mock recorder for MockWsPipelinesLister.
type MockWsPipelinesListerMockRecorder struct {
	mock *MockWsPipelinesLister
}

// NewMockWsPipelinesLister creates a new mock instance.
func NewMockWsPipelinesLister(ctrl *gomock.Controller) *MockWsPipelinesLister {
	mock := &MockWsPipelinesLister{ctrl: ctrl}
	mock.recorder = &MockWsPipelinesListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWsPipelinesLister) EXPECT() *MockWsPipelinesListerMockRecorder {
	return m.recorder
}

// ListPipelines mocks base method.
func (m *MockWsPipelinesLister) ListPipelines() ([]workspace.PipelineManifest, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListPipelines")
	ret0, _ := ret[0].([]workspace.PipelineManifest)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListPipelines indicates an expected call of ListPipelines.
func (mr *MockWsPipelinesListerMockRecorder) ListPipelines() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListPipelines", reflect.TypeOf((*MockWsPipelinesLister)(nil).ListPipelines))
}

// MockCodePipelineLister is a mock of CodePipelineLister interface.
type MockCodePipelineLister struct {
	ctrl     *gomock.Controller
	recorder *MockCodePipelineListerMockRecorder
}

// MockCodePipelineListerMockRecorder is the mock recorder for MockCodePipelineLister.
type MockCodePipelineListerMockRecorder struct {
	mock *MockCodePipelineLister
}

// NewMockCodePipelineLister creates a new mock instance.
func NewMockCodePipelineLister(ctrl *gomock.Controller) *MockCodePipelineLister {
	mock := &MockCodePipelineLister{ctrl: ctrl}
	mock.recorder = &MockCodePipelineListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCodePipelineLister) EXPECT() *MockCodePipelineListerMockRecorder {
	return m.recorder
}

// ListPipelineNamesByTags mocks base method.
func (m *MockCodePipelineLister) ListPipelineNamesByTags(tags map[string]string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListPipelineNamesByTags", tags)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListPipelineNamesByTags indicates an expected call of ListPipelineNamesByTags.
func (mr *MockCodePipelineListerMockRecorder) ListPipelineNamesByTags(tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListPipelineNamesByTags", reflect.TypeOf((*MockCodePipelineLister)(nil).ListPipelineNamesByTags), tags)
}

// MockWorkspaceRetriever is a mock of WorkspaceRetriever interface.
type MockWorkspaceRetriever struct {
	ctrl     *gomock.Controller
	recorder *MockWorkspaceRetrieverMockRecorder
}

// MockWorkspaceRetrieverMockRecorder is the mock recorder for MockWorkspaceRetriever.
type MockWorkspaceRetrieverMockRecorder struct {
	mock *MockWorkspaceRetriever
}

// NewMockWorkspaceRetriever creates a new mock instance.
func NewMockWorkspaceRetriever(ctrl *gomock.Controller) *MockWorkspaceRetriever {
	mock := &MockWorkspaceRetriever{ctrl: ctrl}
	mock.recorder = &MockWorkspaceRetrieverMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockWorkspaceRetriever) EXPECT() *MockWorkspaceRetrieverMockRecorder {
	return m.recorder
}

// ListDockerfiles mocks base method.
func (m *MockWorkspaceRetriever) ListDockerfiles() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListDockerfiles")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListDockerfiles indicates an expected call of ListDockerfiles.
func (mr *MockWorkspaceRetrieverMockRecorder) ListDockerfiles() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListDockerfiles", reflect.TypeOf((*MockWorkspaceRetriever)(nil).ListDockerfiles))
}

// ListJobs mocks base method.
func (m *MockWorkspaceRetriever) ListJobs() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListJobs")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListJobs indicates an expected call of ListJobs.
func (mr *MockWorkspaceRetrieverMockRecorder) ListJobs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListJobs", reflect.TypeOf((*MockWorkspaceRetriever)(nil).ListJobs))
}

// ListServices mocks base method.
func (m *MockWorkspaceRetriever) ListServices() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListServices")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListServices indicates an expected call of ListServices.
func (mr *MockWorkspaceRetrieverMockRecorder) ListServices() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListServices", reflect.TypeOf((*MockWorkspaceRetriever)(nil).ListServices))
}

// ListWorkloads mocks base method.
func (m *MockWorkspaceRetriever) ListWorkloads() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListWorkloads")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListWorkloads indicates an expected call of ListWorkloads.
func (mr *MockWorkspaceRetrieverMockRecorder) ListWorkloads() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListWorkloads", reflect.TypeOf((*MockWorkspaceRetriever)(nil).ListWorkloads))
}

// Summary mocks base method.
func (m *MockWorkspaceRetriever) Summary() (*workspace.Summary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Summary")
	ret0, _ := ret[0].(*workspace.Summary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Summary indicates an expected call of Summary.
func (mr *MockWorkspaceRetrieverMockRecorder) Summary() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Summary", reflect.TypeOf((*MockWorkspaceRetriever)(nil).Summary))
}

// MockDeployStoreClient is a mock of DeployStoreClient interface.
type MockDeployStoreClient struct {
	ctrl     *gomock.Controller
	recorder *MockDeployStoreClientMockRecorder
}

// MockDeployStoreClientMockRecorder is the mock recorder for MockDeployStoreClient.
type MockDeployStoreClientMockRecorder struct {
	mock *MockDeployStoreClient
}

// NewMockDeployStoreClient creates a new mock instance.
func NewMockDeployStoreClient(ctrl *gomock.Controller) *MockDeployStoreClient {
	mock := &MockDeployStoreClient{ctrl: ctrl}
	mock.recorder = &MockDeployStoreClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDeployStoreClient) EXPECT() *MockDeployStoreClientMockRecorder {
	return m.recorder
}

// IsJobDeployed mocks base method.
func (m *MockDeployStoreClient) IsJobDeployed(appName, envName, jobName string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsJobDeployed", appName, envName, jobName)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsJobDeployed indicates an expected call of IsJobDeployed.
func (mr *MockDeployStoreClientMockRecorder) IsJobDeployed(appName, envName, jobName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsJobDeployed", reflect.TypeOf((*MockDeployStoreClient)(nil).IsJobDeployed), appName, envName, jobName)
}

// IsServiceDeployed mocks base method.
func (m *MockDeployStoreClient) IsServiceDeployed(appName, envName, svcName string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsServiceDeployed", appName, envName, svcName)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsServiceDeployed indicates an expected call of IsServiceDeployed.
func (mr *MockDeployStoreClientMockRecorder) IsServiceDeployed(appName, envName, svcName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsServiceDeployed", reflect.TypeOf((*MockDeployStoreClient)(nil).IsServiceDeployed), appName, envName, svcName)
}

// ListDeployedJobs mocks base method.
func (m *MockDeployStoreClient) ListDeployedJobs(appName, envName string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListDeployedJobs", appName, envName)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListDeployedJobs indicates an expected call of ListDeployedJobs.
func (mr *MockDeployStoreClientMockRecorder) ListDeployedJobs(appName, envName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListDeployedJobs", reflect.TypeOf((*MockDeployStoreClient)(nil).ListDeployedJobs), appName, envName)
}

// ListDeployedServices mocks base method.
func (m *MockDeployStoreClient) ListDeployedServices(appName, envName string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListDeployedServices", appName, envName)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListDeployedServices indicates an expected call of ListDeployedServices.
func (mr *MockDeployStoreClientMockRecorder) ListDeployedServices(appName, envName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListDeployedServices", reflect.TypeOf((*MockDeployStoreClient)(nil).ListDeployedServices), appName, envName)
}

// ListSNSTopics mocks base method.
func (m *MockDeployStoreClient) ListSNSTopics(appName, envName string) ([]deploy.Topic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListSNSTopics", appName, envName)
	ret0, _ := ret[0].([]deploy.Topic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListSNSTopics indicates an expected call of ListSNSTopics.
func (mr *MockDeployStoreClientMockRecorder) ListSNSTopics(appName, envName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSNSTopics", reflect.TypeOf((*MockDeployStoreClient)(nil).ListSNSTopics), appName, envName)
}

// MockTaskStackDescriber is a mock of TaskStackDescriber interface.
type MockTaskStackDescriber struct {
	ctrl     *gomock.Controller
	recorder *MockTaskStackDescriberMockRecorder
}

// MockTaskStackDescriberMockRecorder is the mock recorder for MockTaskStackDescriber.
type MockTaskStackDescriberMockRecorder struct {
	mock *MockTaskStackDescriber
}

// NewMockTaskStackDescriber creates a new mock instance.
func NewMockTaskStackDescriber(ctrl *gomock.Controller) *MockTaskStackDescriber {
	mock := &MockTaskStackDescriber{ctrl: ctrl}
	mock.recorder = &MockTaskStackDescriberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskStackDescriber) EXPECT() *MockTaskStackDescriberMockRecorder {
	return m.recorder
}

// ListDefaultTaskStacks mocks base method.
func (m *MockTaskStackDescriber) ListDefaultTaskStacks() ([]deploy.TaskStackInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListDefaultTaskStacks")
	ret0, _ := ret[0].([]deploy.TaskStackInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListDefaultTaskStacks indicates an expected call of ListDefaultTaskStacks.
func (mr *MockTaskStackDescriberMockRecorder) ListDefaultTaskStacks() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListDefaultTaskStacks", reflect.TypeOf((*MockTaskStackDescriber)(nil).ListDefaultTaskStacks))
}

// ListTaskStacks mocks base method.
func (m *MockTaskStackDescriber) ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTaskStacks", appName, envName)
	ret0, _ := ret[0].([]deploy.TaskStackInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTaskStacks indicates an expected call of ListTaskStacks.
func (mr *MockTaskStackDescriberMockRecorder) ListTaskStacks(appName, envName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTaskStacks", reflect.TypeOf((*MockTaskStackDescriber)(nil).ListTaskStacks), appName, envName)
}

// MockTaskLister is a mock of TaskLister interface.
type MockTaskLister struct {
	ctrl     *gomock.Controller
	recorder *MockTaskListerMockRecorder
}

// MockTaskListerMockRecorder is the mock recorder for MockTaskLister.
type MockTaskListerMockRecorder struct {
	mock *MockTaskLister
}

// NewMockTaskLister creates a new mock instance.
func NewMockTaskLister(ctrl *gomock.Controller) *MockTaskLister {
	mock := &MockTaskLister{ctrl: ctrl}
	mock.recorder = &MockTaskListerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskLister) EXPECT() *MockTaskListerMockRecorder {
	return m.recorder
}

// ListActiveAppEnvTasks mocks base method.
func (m *MockTaskLister) ListActiveAppEnvTasks(opts ecs0.ListActiveAppEnvTasksOpts) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListActiveAppEnvTasks", opts)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListActiveAppEnvTasks indicates an expected call of ListActiveAppEnvTasks.
func (mr *MockTaskListerMockRecorder) ListActiveAppEnvTasks(opts interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListActiveAppEnvTasks", reflect.TypeOf((*MockTaskLister)(nil).ListActiveAppEnvTasks), opts)
}

// ListActiveDefaultClusterTasks mocks base method.
func (m *MockTaskLister) ListActiveDefaultClusterTasks(filter ecs0.ListTasksFilter) ([]*ecs.Task, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListActiveDefaultClusterTasks", filter)
	ret0, _ := ret[0].([]*ecs.Task)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListActiveDefaultClusterTasks indicates an expected call of ListActiveDefaultClusterTasks.
func (mr *MockTaskListerMockRecorder) ListActiveDefaultClusterTasks(filter interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListActiveDefaultClusterTasks", reflect.TypeOf((*MockTaskLister)(nil).ListActiveDefaultClusterTasks), filter)
}
