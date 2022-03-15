// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/progress"
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
	when := func(w progress.FileWriter, cf CloudFormation) error {
		return cf.DeployService(w, serviceConfig, "mockBucket")
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
	testCases := map[string]struct {
		in         deploy.DeleteWorkloadInput
		createMock func(ctrl *gomock.Controller) cfnClient
	}{
		"calls delete with the appropriate stack name": {
			in: deploy.DeleteWorkloadInput{
				Name:    "webhook",
				EnvName: "test",
				AppName: "kudos",
			},
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().DeleteAndWait("kudos-test-webhook")
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
			}

			// WHEN
			err := c.DeleteWorkload(tc.in)

			// THEN
			require.NoError(t, err)
		})
	}
}
