// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	sdkcloudformation "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
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
	testCases := map[string]struct {
		wantedErr  string
		createMock func(ctrl *gomock.Controller) cfnClient
	}{
		"does not call update if the stack is new": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				stack := cloudformation.NewStack("webhook", "template",
					cloudformation.WithParameters(map[string]string{
						"port": "80",
					}),
					cloudformation.WithTags(map[string]string{
						"app": "myapp",
					}),
					cloudformation.WithRoleARN("myrole"))
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(stack).Return(nil)
				m.EXPECT().UpdateAndWait(gomock.Any()).Times(0)
				return m
			},
		},
		"calls update if the stack already exists": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(&cloudformation.ErrStackAlreadyExists{
					Name: "name",
				})
				m.EXPECT().UpdateAndWait(gomock.Any())
				return m
			},
		},
		"calls describe if create fails": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().ErrorEvents(gomock.Any()).Return([]cloudformation.StackEvent{
					{ResourceStatusReason: aws.String("Bad things happened. (Service abcd)")},
				}, nil)
				return m
			},
			wantedErr: "some error: Bad things happened",
		},
		"returns descriptive error if describe fails": {
			createMock: func(ctrl *gomock.Controller) cfnClient {
				m := mocks.NewMockcfnClient(ctrl)
				m.EXPECT().CreateAndWait(gomock.Any()).Return(errors.New("some error"))
				m.EXPECT().ErrorEvents(gomock.Any()).Return(nil, errors.New("other error"))
				return m
			},
			wantedErr: "some error: describe stack: other error",
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
			conf := &mockStackConfig{
				name:     "webhook",
				template: "template",
				parameters: map[string]string{
					"port": "80",
				},
				tags: map[string]string{
					"app": "myapp",
				},
			}

			// WHEN
			err := c.DeployService(conf, cloudformation.WithRoleARN("myrole"))

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
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

func TestCloudFormation_GetStackErrors(t *testing.T) {
	mockEvents := []cloudformation.StackEvent{
		{
			LogicalResourceId:    aws.String("abc123"),
			ResourceType:         aws.String("ECS::Service"),
			ResourceStatus:       aws.String("CREATE_FAILED"),
			ResourceStatusReason: aws.String("Space elevator disconnected. (Service moonshot)"),
		},
	}
	testCases := map[string]struct {
		mockCfn      func(*mocks.MockcfnClient)
		wantedErr    string
		wantedEvents []deploy.ResourceEvent
	}{
		"returns successfully": {
			mockCfn: func(m *mocks.MockcfnClient) {
				m.EXPECT().ErrorEvents("myStack").Return(mockEvents, nil)
			},
			wantedEvents: []deploy.ResourceEvent{
				{
					Resource: deploy.Resource{
						LogicalName: "abc123",
						Type:        "ECS::Service",
					},
					Status:       "CREATE_FAILED",
					StatusReason: "Space elevator disconnected",
				},
			},
		},
		"error getting events": {
			mockCfn: func(m *mocks.MockcfnClient) {
				m.EXPECT().ErrorEvents("myStack").Return(nil, errors.New("some error"))
			},
			wantedErr: "some error",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEn
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cfnClient := mocks.NewMockcfnClient(ctrl)
			tc.mockCfn(cfnClient)
			c := CloudFormation{
				cfnClient: cfnClient,
			}

			conf := &mockStackConfig{
				name:     "myStack",
				template: "template",
			}
			// WHEN
			out, err := c.ErrorEvents(conf)

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Equal(t, out, tc.wantedEvents)
			}
		})
	}
}
