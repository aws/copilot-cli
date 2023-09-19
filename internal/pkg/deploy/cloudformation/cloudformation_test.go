// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation/stackset"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockOverrider struct {
	out []byte
	err error
}

func (m *mockOverrider) Override(_ []byte) ([]byte, error) {
	return m.out, m.err
}

func TestWrapWithTemplateOverrider(t *testing.T) {
	t.Run("should return the overriden Template", func(t *testing.T) {
		// GIVEN
		var stack StackConfiguration = &mockStackConfig{template: "hello"}
		ovrdr := &mockOverrider{out: []byte("bye")}

		// WHEN
		stack = WrapWithTemplateOverrider(stack, ovrdr)
		tpl, err := stack.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, "bye", tpl)
	})
	t.Run("should return a wrapped error when Override call fails", func(t *testing.T) {
		// GIVEN
		var stack StackConfiguration = &mockStackConfig{template: "hello"}
		ovrdr := &mockOverrider{err: errors.New("some error")}

		// WHEN
		stack = WrapWithTemplateOverrider(stack, ovrdr)
		_, err := stack.Template()

		// THEN
		require.EqualError(t, err, "override template: some error")
	})
}

func TestIsEmptyErr(t *testing.T) {
	testCases := map[string]struct {
		err    error
		wanted bool
	}{
		"should return true when the error is an ErrStackSetNotFound": {
			err:    &stackset.ErrStackSetNotFound{},
			wanted: true,
		},
		"should return true when the error is an ErrStackSetInstancesNotFound": {
			err:    &stackset.ErrStackSetInstancesNotFound{},
			wanted: true,
		},
		"should return false on any other error": {
			err: errors.New("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, IsEmptyErr(tc.err))
		})
	}
}

type mockFileWriter struct {
	io.Writer
}

func (m mockFileWriter) Fd() uintptr { return 0 }

func testDeployWorkload_OnPushToS3Failure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("some error")
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload("mockBucket", gomock.Any(), gomock.Any()).Return("", wantedErr)

	buf := new(strings.Builder)
	client := CloudFormation{
		s3Client: mS3Client,
		console:  mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), `expected returned error to be wrapped with "some error"`)
}

func testDeployWorkload_OnCreateChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("some error")
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("", wantedErr)
	m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), `expected returned error to be wrapped with "some error"`)
}

func testDeployWorkload_OnUpdateChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("some error")
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("", &cloudformation.ErrStackAlreadyExists{})
	m.EXPECT().Update(gomock.Any()).Return("", wantedErr)
	m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), `expected returned error to be wrapped with "some error"`)
}

func testDeployWorkload_OnDescribeChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(nil, errors.New("DescribeChangeSet error"))
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}
	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, "DescribeChangeSet error")
}

func testDeployWorkload_OnTemplateBodyFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", errors.New("TemplateBody error"))
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, "TemplateBody error")
}

func testDeployWorkload_StackStreamerFailureShouldCancelRenderer(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("streamer error")
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", nil)
	m.EXPECT().DescribeStackEvents(gomock.Any()).Return(nil, wantedErr)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}
	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), "expected streamer error to be wrapped and returned")
}

func testDeployWorkload_StreamUntilStackCreationFails(t *testing.T, stackName string, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil)
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", nil)
	m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_FAILED"), // Send failure event for stack.
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()
	m.EXPECT().Describe(stackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_FAILED"),
	}, nil)
	m.EXPECT().ErrorEvents(stackName).Return(
		[]cloudformation.StackEvent{
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::AppRunner::Service"),
				ResourceStatus:     aws.String("CREATE_FAILED"), // Send failure event for stack.
				Timestamp:          aws.Time(time.Now()),
			},
		}, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, fmt.Sprintf("stack %s did not complete successfully and exited with status CREATE_FAILED", stackName))
}

func testDeployWorkload_RenderNewlyCreatedStackWithECSService(t *testing.T, stackName string, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload("mockBucket", gomock.Any(), gomock.Any()).Return("mockURL", nil)
	mockCFN := mocks.NewMockcfnClient(ctrl)
	mockECS := mocks.NewMockecsClient(ctrl)
	deploymentTime := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)

	mockCFN.EXPECT().Create(gomock.Any()).Return("1234", nil)
	mockCFN.EXPECT().DescribeChangeSet("1234", stackName).Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId: aws.String("Service"),
					ResourceType:      aws.String("AWS::ECS::Service"),
				},
			},
		},
	}, nil)
	mockCFN.EXPECT().TemplateBodyFromChangeSet("1234", stackName).Return(`
Resources:
  Service:
    Metadata:
      'aws:copilot:description': 'My ECS Service'
    Type: AWS::ECS::Service
`, nil)
	mockCFN.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("1"),
				LogicalResourceId:  aws.String("Service"),
				PhysicalResourceId: aws.String("arn:aws:ecs:us-west-2:1111:service/cluster/service"),
				ResourceType:       aws.String("AWS::ECS::Service"),
				ResourceStatus:     aws.String("CREATE_IN_PROGRESS"),
				Timestamp:          aws.Time(deploymentTime),
			},
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String("Service"),
				PhysicalResourceId: aws.String("arn:aws:ecs:us-west-2:1111:service/cluster/service"),
				ResourceType:       aws.String("AWS::ECS::Service"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(deploymentTime),
			},
			{
				EventId:           aws.String("3"),
				LogicalResourceId: aws.String(stackName),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    aws.String("CREATE_COMPLETE"),
				Timestamp:         aws.Time(deploymentTime),
			},
		},
	}, nil).AnyTimes()
	mockECS.EXPECT().Service("cluster", "service").Return(&ecs.Service{
		Deployments: []*awsecs.Deployment{
			{
				RolloutState:   aws.String("COMPLETED"),
				Status:         aws.String("PRIMARY"),
				TaskDefinition: aws.String("arn:aws:ecs:us-west-2:1111:task-definition/hello:10"),
				UpdatedAt:      aws.Time(deploymentTime),
			},
		},
	}, nil)
	mockECS.EXPECT().StoppedServiceTasks("cluster", "service").Return(nil, nil)
	mockCFN.EXPECT().Describe(stackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_COMPLETE"),
	}, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: mockCFN, ecsClient: mockECS, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.NoError(t, err)
	require.Contains(t, buf.String(), "My ECS Service", "resource should be rendered")
	require.Contains(t, buf.String(), "PRIMARY", "Status of the service should be rendered")
	require.Contains(t, buf.String(), "[completed]", "Rollout state of service should be rendered")
}

func testDeployWorkload_WithEnvControllerRenderer_NoStackUpdates(t *testing.T, svcStackName string, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload("mockBucket", gomock.Any(), gomock.Any()).Return("mockURL", nil)
	mockCFN := mocks.NewMockcfnClient(ctrl)
	deploymentTime := time.Date(2020, time.November, 23, 18, 0, 0, 0, time.UTC)

	mockCFN.EXPECT().Create(gomock.Any()).Return("1234", nil)
	mockCFN.EXPECT().DescribeChangeSet("1234", svcStackName).Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId: aws.String("EnvControllerAction"),
					ResourceType:      aws.String("Custom::EnvControllerFunction"),
					Action:            aws.String(sdkcloudformation.ChangeActionAdd),
				},
			},
		},
	}, nil)
	mockCFN.EXPECT().TemplateBodyFromChangeSet("1234", svcStackName).Return(`
Resources:
  EnvControllerAction:
    Metadata:
      'aws:copilot:description': "Updating environment"
`, nil)
	mockCFN.EXPECT().Describe(svcStackName).Return(&cloudformation.StackDescription{
		Tags: []*sdkcloudformation.Tag{
			{
				Key:   aws.String("copilot-application"),
				Value: aws.String("my-app"),
			},
			{
				Key:   aws.String("copilot-environment"),
				Value: aws.String("my-env"),
			},
		},
	}, nil)
	mockCFN.EXPECT().TemplateBody("my-app-my-env").Return(`
Resources:
  PublicLoadBalancer:
    Metadata:
      'aws:copilot:description': "Updating ALB"
`, nil)
	mockCFN.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String(svcStackName),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:           aws.String("1"),
				LogicalResourceId: aws.String(svcStackName),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    aws.String("CREATE_COMPLETE"),
				Timestamp:         aws.Time(deploymentTime),
			},
		},
	}, nil).AnyTimes()
	mockCFN.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String("my-app-my-env"),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{}, // No updates for the env stack.
	}, nil).AnyTimes()

	mockCFN.EXPECT().Describe(svcStackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_COMPLETE"),
	}, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: mockCFN, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Updating environment", "env stack description is rendered")
}

func testDeployWorkload_RenderNewlyCreatedStackWithAddons(t *testing.T, stackName string, when func(cf CloudFormation) error) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockcfnClient(ctrl)
	mS3Client := mocks.NewMocks3Client(ctrl)
	mS3Client.EXPECT().Upload("mockBucket", "manual/templates/myapp-myenv-mysvc/5cde0f1298f41f7d1c8b907a36992a7a513225a2615bd6e307bf1a9149b06b40.yml", gomock.Any()).Return("mockURL", nil)

	// Mocks for the parent stack.
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet("1234", stackName).Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
				},
			},
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					ChangeSetId:        aws.String("5678"),
					LogicalResourceId:  aws.String("AddonsStack"),
					PhysicalResourceId: aws.String("arn:aws:cloudformation:us-west-2:12345:stack/my-nested-stack/d0a825a0-e4cd-xmpl-b9fb-061c69e99205"),
				},
			},
		},
	}, nil)

	m.EXPECT().TemplateBodyFromChangeSet("1234", stackName).Return(`
Resources:
  Cluster:
    Metadata:
      'aws:copilot:description': 'An ECS cluster'
    Type: AWS::ECS::Cluster
  AddonsStack:
    Metadata:
      'aws:copilot:description': 'An Addons CloudFormation Stack for your additional AWS resources'
    Type: AWS::CloudFormation::Stack
`, nil)

	m.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("1"),
				LogicalResourceId:  aws.String("Cluster"),
				PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String("AddonsStack"),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("3"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()

	m.EXPECT().Describe(stackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_COMPLETE"),
	}, nil)

	// Mocks for the addons stack.
	m.EXPECT().DescribeChangeSet("5678", "my-nested-stack").Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId:  aws.String("MyTable"),
					PhysicalResourceId: aws.String("AWS::DynamoDB::Table"),
				},
			},
		},
	}, nil)

	m.EXPECT().TemplateBodyFromChangeSet("5678", "my-nested-stack").Return(`
Resources:
  MyTable:
    Metadata:
      'aws:copilot:description': 'A DynamoDB table to store data'
    Type: AWS::DynamoDB::Table`, nil)

	m.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String("my-nested-stack"),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("1"),
				LogicalResourceId:  aws.String("MyTable"),
				PhysicalResourceId: aws.String("AWS::DynamoDB::Table"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String("my-nested-stack"),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, s3Client: mS3Client, console: mockFileWriter{Writer: buf},
		notifySignals: func() chan os.Signal {
			sigCh := make(chan os.Signal, 1)
			return sigCh
		},
	}

	// WHEN
	err := when(client)

	// THEN
	require.NoError(t, err)
	require.Contains(t, buf.String(), "An ECS cluster")
	require.Contains(t, buf.String(), "An Addons CloudFormation Stack for your additional AWS resources")
	require.Contains(t, buf.String(), "A DynamoDB table to store data")
}

func testDeployTask_OnCreateChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("some error")
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("", wantedErr)
	m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), `expected returned error to be wrapped with "some error"`)
}

func testDeployTask_ReturnNilOnEmptyChangeSetWhileUpdatingStack(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := &cloudformation.ErrChangeSetEmpty{}
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("", &cloudformation.ErrStackAlreadyExists{})
	m.EXPECT().Update(gomock.Any()).Return("", wantedErr)
	m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.Nil(t, err, "should not fail if the changeset is empty")
}

func testDeployTask_OnUpdateChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("some error")
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("", &cloudformation.ErrStackAlreadyExists{})
	m.EXPECT().Update(gomock.Any()).Return("", wantedErr)
	m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), `expected returned error to be wrapped with "some error"`)
}

func testDeployTask_OnDescribeChangeSetFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(nil, errors.New("DescribeChangeSet error"))
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, "DescribeChangeSet error")
}

func testDeployTask_OnTemplateBodyFailure(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", errors.New("TemplateBody error"))
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, "TemplateBody error")
}

func testDeployTask_StackStreamerFailureShouldCancelRenderer(t *testing.T, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	wantedErr := errors.New("streamer error")
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", nil)
	m.EXPECT().DescribeStackEvents(gomock.Any()).Return(nil, wantedErr)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.True(t, errors.Is(err, wantedErr), "expected streamer error to be wrapped and returned")
}

func testDeployTask_StreamUntilStackCreationFails(t *testing.T, stackName string, when func(cf CloudFormation) error) {
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockcfnClient(ctrl)
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet(gomock.Any(), gomock.Any()).Return(&cloudformation.ChangeSetDescription{}, nil)
	m.EXPECT().TemplateBodyFromChangeSet(gomock.Any(), gomock.Any()).Return("", nil)
	m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_FAILED"), // Send failure event for stack.
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()
	m.EXPECT().Describe(stackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_FAILED"),
	}, nil)
	m.EXPECT().ErrorEvents(stackName).Return(
		[]cloudformation.StackEvent{
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::AppRunner::Service"),
				ResourceStatus:     aws.String("CREATE_FAILED"), // Send failure event for stack.
				Timestamp:          aws.Time(time.Now()),
			},
		}, nil)
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.EqualError(t, err, fmt.Sprintf("stack %s did not complete successfully and exited with status CREATE_FAILED", stackName))
}

func testDeployTask_RenderNewlyCreatedStackWithAddons(t *testing.T, stackName string, when func(cf CloudFormation) error) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockcfnClient(ctrl)

	// Mocks for the parent stack.
	m.EXPECT().Create(gomock.Any()).Return("1234", nil)
	m.EXPECT().DescribeChangeSet("1234", stackName).Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId:  aws.String("Cluster"),
					PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
				},
			},
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					ChangeSetId:        aws.String("5678"),
					LogicalResourceId:  aws.String("AddonsStack"),
					PhysicalResourceId: aws.String("arn:aws:cloudformation:us-west-2:12345:stack/my-nested-stack/d0a825a0-e4cd-xmpl-b9fb-061c69e99205"),
				},
			},
		},
	}, nil)

	m.EXPECT().TemplateBodyFromChangeSet("1234", stackName).Return(`
Resources:
  Cluster:
    Metadata:
      'aws:copilot:description': 'An ECS cluster'
    Type: AWS::ECS::Cluster
  AddonsStack:
    Metadata:
      'aws:copilot:description': 'An Addons CloudFormation Stack for your additional AWS resources'
    Type: AWS::CloudFormation::Stack
`, nil)

	m.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String(stackName),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("1"),
				LogicalResourceId:  aws.String("Cluster"),
				PhysicalResourceId: aws.String("AWS::ECS::Cluster"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String("AddonsStack"),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("3"),
				LogicalResourceId:  aws.String(stackName),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()

	m.EXPECT().Describe(stackName).Return(&cloudformation.StackDescription{
		StackStatus: aws.String("CREATE_COMPLETE"),
	}, nil)

	// Mocks for the addons stack.
	m.EXPECT().DescribeChangeSet("5678", "my-nested-stack").Return(&cloudformation.ChangeSetDescription{
		Changes: []*sdkcloudformation.Change{
			{
				ResourceChange: &sdkcloudformation.ResourceChange{
					LogicalResourceId:  aws.String("MyTable"),
					PhysicalResourceId: aws.String("AWS::DynamoDB::Table"),
				},
			},
		},
	}, nil)

	m.EXPECT().TemplateBodyFromChangeSet("5678", "my-nested-stack").Return(`
Resources:
  MyTable:
    Metadata:
      'aws:copilot:description': 'A DynamoDB table to store data'
    Type: AWS::DynamoDB::Table`, nil)

	m.EXPECT().DescribeStackEvents(&sdkcloudformation.DescribeStackEventsInput{
		StackName: aws.String("my-nested-stack"),
	}).Return(&sdkcloudformation.DescribeStackEventsOutput{
		StackEvents: []*sdkcloudformation.StackEvent{
			{
				EventId:            aws.String("1"),
				LogicalResourceId:  aws.String("MyTable"),
				PhysicalResourceId: aws.String("AWS::DynamoDB::Table"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
			{
				EventId:            aws.String("2"),
				LogicalResourceId:  aws.String("my-nested-stack"),
				PhysicalResourceId: aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:     aws.String("CREATE_COMPLETE"),
				Timestamp:          aws.Time(time.Now()),
			},
		},
	}, nil).AnyTimes()
	buf := new(strings.Builder)
	client := CloudFormation{cfnClient: m, console: mockFileWriter{Writer: buf}}

	// WHEN
	err := when(client)

	// THEN
	require.NoError(t, err)
	require.Contains(t, buf.String(), "An ECS cluster")
	require.Contains(t, buf.String(), "An Addons CloudFormation Stack for your additional AWS resources")
	require.Contains(t, buf.String(), "A DynamoDB table to store data")
}

func TestCloudFormation_Template(t *testing.T) {
	inStackName := stack.NameForEnv("phonetool", "test")
	testCases := map[string]struct {
		inClient       func(ctrl *gomock.Controller) *mocks.MockcfnClient
		wantedTemplate string
		wantedError    error
	}{
		"error getting the template body": {
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody("phonetool-test").Return("", errors.New("some error"))
				return m
			},
			wantedError: errors.New("some error"),
		},
		"returns the template body": {
			inClient: func(ctrl *gomock.Controller) *mocks.MockcfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody("phonetool-test").Return("mockTemplate", nil)
				return m
			},
			wantedTemplate: "mockTemplate",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cf := &CloudFormation{
				cfnClient: tc.inClient(ctrl),
			}

			// WHEN
			got, gotErr := cf.Template(inStackName)
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantedTemplate, got)
			}
		})
	}
}
