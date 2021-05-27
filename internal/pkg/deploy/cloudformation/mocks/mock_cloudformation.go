// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/pkg/deploy/cloudformation/cloudformation.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	io "io"
	reflect "reflect"

	cloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	cloudformation0 "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	stackset "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	ecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	gomock "github.com/golang/mock/gomock"
)

// MockStackConfiguration is a mock of StackConfiguration interface.
type MockStackConfiguration struct {
	ctrl     *gomock.Controller
	recorder *MockStackConfigurationMockRecorder
}

// MockStackConfigurationMockRecorder is the mock recorder for MockStackConfiguration.
type MockStackConfigurationMockRecorder struct {
	mock *MockStackConfiguration
}

// NewMockStackConfiguration creates a new mock instance.
func NewMockStackConfiguration(ctrl *gomock.Controller) *MockStackConfiguration {
	mock := &MockStackConfiguration{ctrl: ctrl}
	mock.recorder = &MockStackConfigurationMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStackConfiguration) EXPECT() *MockStackConfigurationMockRecorder {
	return m.recorder
}

// Parameters mocks base method.
func (m *MockStackConfiguration) Parameters() ([]*cloudformation.Parameter, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parameters")
	ret0, _ := ret[0].([]*cloudformation.Parameter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parameters indicates an expected call of Parameters.
func (mr *MockStackConfigurationMockRecorder) Parameters() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parameters", reflect.TypeOf((*MockStackConfiguration)(nil).Parameters))
}

// StackName mocks base method.
func (m *MockStackConfiguration) StackName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StackName")
	ret0, _ := ret[0].(string)
	return ret0
}

// StackName indicates an expected call of StackName.
func (mr *MockStackConfigurationMockRecorder) StackName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StackName", reflect.TypeOf((*MockStackConfiguration)(nil).StackName))
}

// Tags mocks base method.
func (m *MockStackConfiguration) Tags() []*cloudformation.Tag {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Tags")
	ret0, _ := ret[0].([]*cloudformation.Tag)
	return ret0
}

// Tags indicates an expected call of Tags.
func (mr *MockStackConfigurationMockRecorder) Tags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Tags", reflect.TypeOf((*MockStackConfiguration)(nil).Tags))
}

// Template mocks base method.
func (m *MockStackConfiguration) Template() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Template")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Template indicates an expected call of Template.
func (mr *MockStackConfigurationMockRecorder) Template() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Template", reflect.TypeOf((*MockStackConfiguration)(nil).Template))
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

// MockcfnClient is a mock of cfnClient interface.
type MockcfnClient struct {
	ctrl     *gomock.Controller
	recorder *MockcfnClientMockRecorder
}

// MockcfnClientMockRecorder is the mock recorder for MockcfnClient.
type MockcfnClientMockRecorder struct {
	mock *MockcfnClient
}

// NewMockcfnClient creates a new mock instance.
func NewMockcfnClient(ctrl *gomock.Controller) *MockcfnClient {
	mock := &MockcfnClient{ctrl: ctrl}
	mock.recorder = &MockcfnClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockcfnClient) EXPECT() *MockcfnClientMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockcfnClient) Create(arg0 *cloudformation0.Stack) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockcfnClientMockRecorder) Create(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockcfnClient)(nil).Create), arg0)
}

// CreateAndWait mocks base method.
func (m *MockcfnClient) CreateAndWait(arg0 *cloudformation0.Stack) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateAndWait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateAndWait indicates an expected call of CreateAndWait.
func (mr *MockcfnClientMockRecorder) CreateAndWait(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateAndWait", reflect.TypeOf((*MockcfnClient)(nil).CreateAndWait), arg0)
}

// Delete mocks base method.
func (m *MockcfnClient) Delete(stackName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", stackName)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockcfnClientMockRecorder) Delete(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockcfnClient)(nil).Delete), stackName)
}

// DeleteAndWait mocks base method.
func (m *MockcfnClient) DeleteAndWait(stackName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAndWait", stackName)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAndWait indicates an expected call of DeleteAndWait.
func (mr *MockcfnClientMockRecorder) DeleteAndWait(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAndWait", reflect.TypeOf((*MockcfnClient)(nil).DeleteAndWait), stackName)
}

// DeleteAndWaitWithRoleARN mocks base method.
func (m *MockcfnClient) DeleteAndWaitWithRoleARN(stackName, roleARN string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAndWaitWithRoleARN", stackName, roleARN)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAndWaitWithRoleARN indicates an expected call of DeleteAndWaitWithRoleARN.
func (mr *MockcfnClientMockRecorder) DeleteAndWaitWithRoleARN(stackName, roleARN interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAndWaitWithRoleARN", reflect.TypeOf((*MockcfnClient)(nil).DeleteAndWaitWithRoleARN), stackName, roleARN)
}

// Describe mocks base method.
func (m *MockcfnClient) Describe(stackName string) (*cloudformation0.StackDescription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Describe", stackName)
	ret0, _ := ret[0].(*cloudformation0.StackDescription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Describe indicates an expected call of Describe.
func (mr *MockcfnClientMockRecorder) Describe(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Describe", reflect.TypeOf((*MockcfnClient)(nil).Describe), stackName)
}

// DescribeChangeSet mocks base method.
func (m *MockcfnClient) DescribeChangeSet(changeSetID, stackName string) (*cloudformation0.ChangeSetDescription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeChangeSet", changeSetID, stackName)
	ret0, _ := ret[0].(*cloudformation0.ChangeSetDescription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeChangeSet indicates an expected call of DescribeChangeSet.
func (mr *MockcfnClientMockRecorder) DescribeChangeSet(changeSetID, stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeChangeSet", reflect.TypeOf((*MockcfnClient)(nil).DescribeChangeSet), changeSetID, stackName)
}

// DescribeStackEvents mocks base method.
func (m *MockcfnClient) DescribeStackEvents(arg0 *cloudformation.DescribeStackEventsInput) (*cloudformation.DescribeStackEventsOutput, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeStackEvents", arg0)
	ret0, _ := ret[0].(*cloudformation.DescribeStackEventsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DescribeStackEvents indicates an expected call of DescribeStackEvents.
func (mr *MockcfnClientMockRecorder) DescribeStackEvents(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeStackEvents", reflect.TypeOf((*MockcfnClient)(nil).DescribeStackEvents), arg0)
}

// ErrorEvents mocks base method.
func (m *MockcfnClient) ErrorEvents(stackName string) ([]cloudformation0.StackEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ErrorEvents", stackName)
	ret0, _ := ret[0].([]cloudformation0.StackEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ErrorEvents indicates an expected call of ErrorEvents.
func (mr *MockcfnClientMockRecorder) ErrorEvents(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ErrorEvents", reflect.TypeOf((*MockcfnClient)(nil).ErrorEvents), stackName)
}

// Events mocks base method.
func (m *MockcfnClient) Events(stackName string) ([]cloudformation0.StackEvent, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Events", stackName)
	ret0, _ := ret[0].([]cloudformation0.StackEvent)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Events indicates an expected call of Events.
func (mr *MockcfnClientMockRecorder) Events(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Events", reflect.TypeOf((*MockcfnClient)(nil).Events), stackName)
}

// ListStacksWithTags mocks base method.
func (m *MockcfnClient) ListStacksWithTags(tags map[string]string) ([]cloudformation0.StackDescription, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListStacksWithTags", tags)
	ret0, _ := ret[0].([]cloudformation0.StackDescription)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListStacksWithTags indicates an expected call of ListStacksWithTags.
func (mr *MockcfnClientMockRecorder) ListStacksWithTags(tags interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListStacksWithTags", reflect.TypeOf((*MockcfnClient)(nil).ListStacksWithTags), tags)
}

// Outputs mocks base method.
func (m *MockcfnClient) Outputs(stack *cloudformation0.Stack) (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Outputs", stack)
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Outputs indicates an expected call of Outputs.
func (mr *MockcfnClientMockRecorder) Outputs(stack interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Outputs", reflect.TypeOf((*MockcfnClient)(nil).Outputs), stack)
}

// Parameters mocks base method.
func (m *MockcfnClient) Parameters(stackName string) (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parameters", stackName)
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parameters indicates an expected call of Parameters.
func (mr *MockcfnClientMockRecorder) Parameters(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parameters", reflect.TypeOf((*MockcfnClient)(nil).Parameters), stackName)
}

// TemplateBody mocks base method.
func (m *MockcfnClient) TemplateBody(stackName string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TemplateBody", stackName)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TemplateBody indicates an expected call of TemplateBody.
func (mr *MockcfnClientMockRecorder) TemplateBody(stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TemplateBody", reflect.TypeOf((*MockcfnClient)(nil).TemplateBody), stackName)
}

// TemplateBodyFromChangeSet mocks base method.
func (m *MockcfnClient) TemplateBodyFromChangeSet(changeSetID, stackName string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TemplateBodyFromChangeSet", changeSetID, stackName)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TemplateBodyFromChangeSet indicates an expected call of TemplateBodyFromChangeSet.
func (mr *MockcfnClientMockRecorder) TemplateBodyFromChangeSet(changeSetID, stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TemplateBodyFromChangeSet", reflect.TypeOf((*MockcfnClient)(nil).TemplateBodyFromChangeSet), changeSetID, stackName)
}

// Update mocks base method.
func (m *MockcfnClient) Update(arg0 *cloudformation0.Stack) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockcfnClientMockRecorder) Update(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockcfnClient)(nil).Update), arg0)
}

// UpdateAndWait mocks base method.
func (m *MockcfnClient) UpdateAndWait(arg0 *cloudformation0.Stack) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateAndWait", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateAndWait indicates an expected call of UpdateAndWait.
func (mr *MockcfnClientMockRecorder) UpdateAndWait(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAndWait", reflect.TypeOf((*MockcfnClient)(nil).UpdateAndWait), arg0)
}

// WaitForCreate mocks base method.
func (m *MockcfnClient) WaitForCreate(ctx context.Context, stackName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForCreate", ctx, stackName)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitForCreate indicates an expected call of WaitForCreate.
func (mr *MockcfnClientMockRecorder) WaitForCreate(ctx, stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForCreate", reflect.TypeOf((*MockcfnClient)(nil).WaitForCreate), ctx, stackName)
}

// WaitForUpdate mocks base method.
func (m *MockcfnClient) WaitForUpdate(ctx context.Context, stackName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForUpdate", ctx, stackName)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitForUpdate indicates an expected call of WaitForUpdate.
func (mr *MockcfnClientMockRecorder) WaitForUpdate(ctx, stackName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForUpdate", reflect.TypeOf((*MockcfnClient)(nil).WaitForUpdate), ctx, stackName)
}

// MockcodeStarClient is a mock of codeStarClient interface.
type MockcodeStarClient struct {
	ctrl     *gomock.Controller
	recorder *MockcodeStarClientMockRecorder
}

// MockcodeStarClientMockRecorder is the mock recorder for MockcodeStarClient.
type MockcodeStarClientMockRecorder struct {
	mock *MockcodeStarClient
}

// NewMockcodeStarClient creates a new mock instance.
func NewMockcodeStarClient(ctrl *gomock.Controller) *MockcodeStarClient {
	mock := &MockcodeStarClient{ctrl: ctrl}
	mock.recorder = &MockcodeStarClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockcodeStarClient) EXPECT() *MockcodeStarClientMockRecorder {
	return m.recorder
}

// WaitUntilConnectionStatusAvailable mocks base method.
func (m *MockcodeStarClient) WaitUntilConnectionStatusAvailable(ctx context.Context, connectionARN string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitUntilConnectionStatusAvailable", ctx, connectionARN)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitUntilConnectionStatusAvailable indicates an expected call of WaitUntilConnectionStatusAvailable.
func (mr *MockcodeStarClientMockRecorder) WaitUntilConnectionStatusAvailable(ctx, connectionARN interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitUntilConnectionStatusAvailable", reflect.TypeOf((*MockcodeStarClient)(nil).WaitUntilConnectionStatusAvailable), ctx, connectionARN)
}

// MockcodePipelineClient is a mock of codePipelineClient interface.
type MockcodePipelineClient struct {
	ctrl     *gomock.Controller
	recorder *MockcodePipelineClientMockRecorder
}

// MockcodePipelineClientMockRecorder is the mock recorder for MockcodePipelineClient.
type MockcodePipelineClientMockRecorder struct {
	mock *MockcodePipelineClient
}

// NewMockcodePipelineClient creates a new mock instance.
func NewMockcodePipelineClient(ctrl *gomock.Controller) *MockcodePipelineClient {
	mock := &MockcodePipelineClient{ctrl: ctrl}
	mock.recorder = &MockcodePipelineClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockcodePipelineClient) EXPECT() *MockcodePipelineClientMockRecorder {
	return m.recorder
}

// RetryStageExecution mocks base method.
func (m *MockcodePipelineClient) RetryStageExecution(pipelineName, stageName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RetryStageExecution", pipelineName, stageName)
	ret0, _ := ret[0].(error)
	return ret0
}

// RetryStageExecution indicates an expected call of RetryStageExecution.
func (mr *MockcodePipelineClientMockRecorder) RetryStageExecution(pipelineName, stageName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RetryStageExecution", reflect.TypeOf((*MockcodePipelineClient)(nil).RetryStageExecution), pipelineName, stageName)
}

// Mocks3Client is a mock of s3Client interface.
type Mocks3Client struct {
	ctrl     *gomock.Controller
	recorder *Mocks3ClientMockRecorder
}

// Mocks3ClientMockRecorder is the mock recorder for Mocks3Client.
type Mocks3ClientMockRecorder struct {
	mock *Mocks3Client
}

// NewMocks3Client creates a new mock instance.
func NewMocks3Client(ctrl *gomock.Controller) *Mocks3Client {
	mock := &Mocks3Client{ctrl: ctrl}
	mock.recorder = &Mocks3ClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mocks3Client) EXPECT() *Mocks3ClientMockRecorder {
	return m.recorder
}

// PutArtifact mocks base method.
func (m *Mocks3Client) PutArtifact(bucket, fileName string, data io.Reader) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PutArtifact", bucket, fileName, data)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutArtifact indicates an expected call of PutArtifact.
func (mr *Mocks3ClientMockRecorder) PutArtifact(bucket, fileName, data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PutArtifact", reflect.TypeOf((*Mocks3Client)(nil).PutArtifact), bucket, fileName, data)
}

// MockstackSetClient is a mock of stackSetClient interface.
type MockstackSetClient struct {
	ctrl     *gomock.Controller
	recorder *MockstackSetClientMockRecorder
}

// MockstackSetClientMockRecorder is the mock recorder for MockstackSetClient.
type MockstackSetClientMockRecorder struct {
	mock *MockstackSetClient
}

// NewMockstackSetClient creates a new mock instance.
func NewMockstackSetClient(ctrl *gomock.Controller) *MockstackSetClient {
	mock := &MockstackSetClient{ctrl: ctrl}
	mock.recorder = &MockstackSetClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockstackSetClient) EXPECT() *MockstackSetClientMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockstackSetClient) Create(name, template string, opts ...stackset.CreateOrUpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{name, template}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockstackSetClientMockRecorder) Create(name, template interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name, template}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockstackSetClient)(nil).Create), varargs...)
}

// CreateInstancesAndWait mocks base method.
func (m *MockstackSetClient) CreateInstancesAndWait(name string, accounts, regions []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateInstancesAndWait", name, accounts, regions)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateInstancesAndWait indicates an expected call of CreateInstancesAndWait.
func (mr *MockstackSetClientMockRecorder) CreateInstancesAndWait(name, accounts, regions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateInstancesAndWait", reflect.TypeOf((*MockstackSetClient)(nil).CreateInstancesAndWait), name, accounts, regions)
}

// Delete mocks base method.
func (m *MockstackSetClient) Delete(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockstackSetClientMockRecorder) Delete(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockstackSetClient)(nil).Delete), name)
}

// Describe mocks base method.
func (m *MockstackSetClient) Describe(name string) (stackset.Description, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Describe", name)
	ret0, _ := ret[0].(stackset.Description)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Describe indicates an expected call of Describe.
func (mr *MockstackSetClientMockRecorder) Describe(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Describe", reflect.TypeOf((*MockstackSetClient)(nil).Describe), name)
}

// InstanceSummaries mocks base method.
func (m *MockstackSetClient) InstanceSummaries(name string, opts ...stackset.InstanceSummariesOption) ([]stackset.InstanceSummary, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{name}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "InstanceSummaries", varargs...)
	ret0, _ := ret[0].([]stackset.InstanceSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InstanceSummaries indicates an expected call of InstanceSummaries.
func (mr *MockstackSetClientMockRecorder) InstanceSummaries(name interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InstanceSummaries", reflect.TypeOf((*MockstackSetClient)(nil).InstanceSummaries), varargs...)
}

// UpdateAndWait mocks base method.
func (m *MockstackSetClient) UpdateAndWait(name, template string, opts ...stackset.CreateOrUpdateOption) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{name, template}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateAndWait", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateAndWait indicates an expected call of UpdateAndWait.
func (mr *MockstackSetClientMockRecorder) UpdateAndWait(name, template interface{}, opts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{name, template}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAndWait", reflect.TypeOf((*MockstackSetClient)(nil).UpdateAndWait), varargs...)
}

// WaitForStackSetLastOperationComplete mocks base method.
func (m *MockstackSetClient) WaitForStackSetLastOperationComplete(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForStackSetLastOperationComplete", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitForStackSetLastOperationComplete indicates an expected call of WaitForStackSetLastOperationComplete.
func (mr *MockstackSetClientMockRecorder) WaitForStackSetLastOperationComplete(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForStackSetLastOperationComplete", reflect.TypeOf((*MockstackSetClient)(nil).WaitForStackSetLastOperationComplete), name)
}
