// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockStackConfig struct {
	name       string
	template   string
	tags       map[string]string
	parameters map[string]string
}

func (m *mockStackConfig) StackName() string {
	return m.name
}

func (m *mockStackConfig) Template() (string, error) {
	return m.template, nil
}

func (m *mockStackConfig) Parameters() ([]*sdkcloudformation.Parameter, error) {
	var params []*sdkcloudformation.Parameter
	for k, v := range m.parameters {
		params = append(params, &sdkcloudformation.Parameter{
			ParameterKey:   aws.String(k),
			ParameterValue: aws.String(v),
		})
	}
	return params, nil
}

func (m *mockStackConfig) SerializedParameters() (string, error) {
	return "", nil
}

func (m *mockStackConfig) Tags() []*sdkcloudformation.Tag {
	var tags []*sdkcloudformation.Tag
	for k, v := range m.tags {
		tags = append(tags, &sdkcloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	return tags
}

func TestCloudFormation_DeployService(t *testing.T) {
	serviceConfig := &mockStackConfig{
		name:     "myapp-myenv-mysvc",
		template: "template",
		parameters: map[string]string{
			"port": "80",
		},
		tags: map[string]string{
			"app": "myapp",
		},
	}
	when := func(cf CloudFormation) error {
		return cf.DeployService(serviceConfig, "mockBucket", false)
	}

	t.Run("returns a wrapped error if pushing to s3 bucket fails", func(t *testing.T) {
		testDeployWorkload_OnPushToS3Failure(t, when)
	})

	t.Run("returns a wrapped error if creating a change set fails", func(t *testing.T) {
		testDeployWorkload_OnCreateChangeSetFailure(t, when)
	})
	t.Run("calls Update if stack is already created and returns wrapped error if Update fails", func(t *testing.T) {
		testDeployWorkload_OnUpdateChangeSetFailure(t, when)
	})
	t.Run("returns an error when the ChangeSet cannot be described for stack changes before rendering", func(t *testing.T) {
		testDeployWorkload_OnDescribeChangeSetFailure(t, when)
	})
	t.Run("returns an error when stack template body cannot be retrieved to parse resource descriptions", func(t *testing.T) {
		testDeployWorkload_OnTemplateBodyFailure(t, when)
	})
	t.Run("returns a wrapped error if a streamer fails and cancels the renderer", func(t *testing.T) {
		testDeployWorkload_StackStreamerFailureShouldCancelRenderer(t, when)
	})
	t.Run("returns an error if stack creation fails", func(t *testing.T) {
		testDeployWorkload_StreamUntilStackCreationFails(t, "myapp-myenv-mysvc", when)
	})
	t.Run("renders a stack with an EnvController that triggers no Env Stack updates", func(t *testing.T) {
		testDeployWorkload_WithEnvControllerRenderer_NoStackUpdates(t, "myapp-myenv-mysvc", when)
	})
	t.Run("renders a stack with an ECS service", func(t *testing.T) {
		testDeployWorkload_RenderNewlyCreatedStackWithECSService(t, "myapp-myenv-mysvc", when)
	})
	t.Run("renders a stack with addons template if stack creation is successful", func(t *testing.T) {
		testDeployWorkload_RenderNewlyCreatedStackWithAddons(t, "myapp-myenv-mysvc", when)
	})
}

func TestCloudFormation_DeleteWorkload(t *testing.T) {
	in := deploy.DeleteWorkloadInput{
		Name:    "webhook",
		EnvName: "test",
		AppName: "kudos",
	}
	testCases := map[string]struct {
		createMock func(ctrl *gomock.Controller) cfnClient
		wanted     error
	}{
		"should short-circuit if the stack is already deleted when retrieving the template body": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", &cloudformation.ErrStackNotFound{})
				return m
			},
		},
		"should return a wrapped error if retrieving the template body fails unexpectedly": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", errors.New("some error"))
				return m
			},
			wanted: errors.New(`get template body of stack "kudos-test-webhook": some error`),
		},
		"should short-circuit if stack is deleted while retrieving the stack ID": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(nil, &cloudformation.ErrStackNotFound{})
				return m
			},
		},
		"should return a wrapped error if retrieving the stack ID fails unexpectedly": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wanted: errors.New(`retrieve the stack ID for stack "kudos-test-webhook": some error`),
		},
		"should return the error as is if the deletion function fails unexpectedly": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
					StackId: aws.String("stack/webhook/1111"),
				}, nil)
				m.EXPECT().DeleteAndWaitWithRoleARN(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{}, nil).AnyTimes()
				return m
			},
			wanted: errors.New("some error"),
		},
		"should return the error as is if the progress renderer fails unexpectedly": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
					StackId: aws.String("stack/webhook/1111"),
				}, nil)
				m.EXPECT().DeleteAndWaitWithRoleARN(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(nil, errors.New("some error"))
				return m
			},
			wanted: errors.New("describe stack events stack/webhook/1111: some error"),
		},
		"should return nil if the deletion function tries to delete an already deleted stack": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().TemplateBody(gomock.Any()).Return("", nil)
				m.EXPECT().Describe(gomock.Any()).Return(&cloudformation.StackDescription{
					StackId: aws.String("stack/webhook/1111"),
				}, nil)
				m.EXPECT().DeleteAndWaitWithRoleARN(gomock.Any(), gomock.Any()).Return(&cloudformation.ErrStackNotFound{})
				m.EXPECT().DescribeStackEvents(gomock.Any()).Return(&sdkcloudformation.DescribeStackEventsOutput{}, nil).AnyTimes()
				return m
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := CloudFormation{
				cfnClient: tc.createMock(ctrl),
				console:   new(discardFile),
			}

			// WHEN
			err := c.DeleteWorkload(in)

			// THEN
			if tc.wanted != nil {
				require.EqualError(t, err, tc.wanted.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
