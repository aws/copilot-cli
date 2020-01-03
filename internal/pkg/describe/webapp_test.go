// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWebAppURI_String(t *testing.T) {
	testCases := map[string]struct {
		dnsName string
		path    string

		wanted string
	}{
		"http": {
			dnsName: "abc.us-west-1.elb.amazonaws.com",
			path:    "*",

			wanted: "http://abc.us-west-1.elb.amazonaws.com and path *",
		},
		"https": {
			dnsName: "jobs.test.phonetool.com",
			path:    "",

			wanted: "https://jobs.test.phonetool.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri := &WebAppURI{
				DNSName: tc.dnsName,
				Path:    tc.path,
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

func TestWebAppDescriber_URI(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
		testEnvSubdomain   = "test.phonetool.com"
		testEnvLBDNSName   = "http://abc.us-west-1.elb.amazonaws.com"
		testAppPath        = "*"
	)
	testCases := map[string]struct {
		mockStore           func(ctrl *gomock.Controller) *mocks.MockenvGetter
		mockStackDescribers func(ctrl *gomock.Controller) map[string]stackDescriber

		wantedURI   *WebAppURI
		wantedError error
	}{
		"environment does not exist in store": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(nil, errors.New("some error"))
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				return nil
			},
			wantedError: errors.New("some error"),
		},
		"cfn error": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(nil, errors.New("some error"))
				describers[testManagerRoleARN] = m
				return describers
			},
			wantedError: fmt.Errorf("describe stack %s: %s", stack.NameForEnv(testProject, testEnv), "some error"),
		},
		"stack does not exist": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStacks(gomock.Any()).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{},
				}, nil)
				describers[testManagerRoleARN] = m
				return describers
			},
			wantedError: fmt.Errorf("stack %s not found", stack.NameForEnv(testProject, testEnv)),
		},
		"https web application": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(stack.NameForEnv(testProject, testEnv)),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							Outputs: []*cloudformation.Output{
								{
									OutputKey:   aws.String(stack.EnvOutputSubdomain),
									OutputValue: aws.String(testEnvSubdomain),
								},
								{
									OutputKey:   aws.String(stack.EnvOutputPublicLoadBalancerDNSName),
									OutputValue: aws.String(testEnvLBDNSName),
								},
							},
						},
					},
				}, nil)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							Parameters: []*cloudformation.Parameter{
								{
									ParameterKey:   aws.String(stack.LBFargateRulePathKey),
									ParameterValue: aws.String(testAppPath),
								},
							},
						},
					},
				}, nil)
				describers[testManagerRoleARN] = m
				return describers
			},

			wantedURI: &WebAppURI{
				DNSName: testApp + "." + testEnvSubdomain,
			},
		},
		"http web application": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(stack.NameForEnv(testProject, testEnv)),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							Outputs: []*cloudformation.Output{
								{
									OutputKey:   aws.String(stack.EnvOutputPublicLoadBalancerDNSName),
									OutputValue: aws.String(testEnvLBDNSName),
								},
							},
						},
					},
				}, nil)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							Parameters: []*cloudformation.Parameter{
								{
									ParameterKey:   aws.String(stack.LBFargateRulePathKey),
									ParameterValue: aws.String(testAppPath),
								},
							},
						},
					},
				}, nil)
				describers[testManagerRoleARN] = m
				return describers
			},

			wantedURI: &WebAppURI{
				DNSName: testEnvLBDNSName,
				Path:    testAppPath,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				store:           tc.mockStore(ctrl),
				stackDescribers: tc.mockStackDescribers(ctrl),
			}

			// WHEN
			actual, err := d.URI(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedURI, actual)
			}
		})
	}
}

func TestWebAppDescriber_ECSParams(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
		testCPU            = "256"
		testMemory         = "512"
		testPort           = "8080"
		testTasks          = "3"
	)
	testCases := map[string]struct {
		mockStore           func(ctrl *gomock.Controller) *mocks.MockenvGetter
		mockStackDescribers func(ctrl *gomock.Controller) map[string]stackDescriber

		wantedECSParams *WebAppECSParams
		wantedError     error
	}{
		"get web application deploy info": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStacks(&cloudformation.DescribeStacksInput{
					StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
				}).Return(&cloudformation.DescribeStacksOutput{
					Stacks: []*cloudformation.Stack{
						{
							Parameters: []*cloudformation.Parameter{
								{
									ParameterKey:   aws.String(stack.LBFargateTaskCPUKey),
									ParameterValue: aws.String(testCPU),
								},
								{
									ParameterKey:   aws.String(stack.LBFargateTaskMemoryKey),
									ParameterValue: aws.String(testMemory),
								},
								{
									ParameterKey:   aws.String(stack.LBFargateParamContainerPortKey),
									ParameterValue: aws.String(testPort),
								},
								{
									ParameterKey:   aws.String(stack.LBFargateTaskCountKey),
									ParameterValue: aws.String(testTasks),
								},
							},
						},
					},
				}, nil)
				describers[testManagerRoleARN] = m
				return describers
			},

			wantedECSParams: &WebAppECSParams{
				ContainerPort: testPort,
				TaskSize: TaskSize{
					CPU:    testCPU,
					Memory: testMemory,
				},
				TaskCount: testTasks,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				store:           tc.mockStore(ctrl),
				stackDescribers: tc.mockStackDescribers(ctrl),
			}

			// WHEN
			actual, err := d.ECSParams(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedECSParams, actual)
			}
		})
	}
}

func TestWebAppDescriber_StackResources(t *testing.T) {
	const (
		testProject        = "phonetool"
		testEnv            = "test"
		testManagerRoleARN = "arn:aws:iam::1111:role/manager"
		testApp            = "jobs"
	)
	testCfnResources := []*CfnResource{
		&CfnResource{
			Type:       "AWS::EC2::SecurityGroup",
			PhysicalID: "sg-0758ed6b233743530",
		},
	}
	testCases := map[string]struct {
		mockStore           func(ctrl *gomock.Controller) *mocks.MockenvGetter
		mockStackDescribers func(ctrl *gomock.Controller) map[string]stackDescriber

		wantedResources []*CfnResource
		wantedError     error
	}{
		"get stack resources": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
					StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
				}).Return(&cloudformation.DescribeStackResourcesOutput{
					StackResources: []*cloudformation.StackResource{
						&cloudformation.StackResource{
							ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
							PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
						},
					},
				}, nil)
				describers[testManagerRoleARN] = m
				return describers
			},

			wantedResources: testCfnResources,
		},
		"returns error when fail to describe stack resources": {
			mockStore: func(ctrl *gomock.Controller) *mocks.MockenvGetter {
				m := mocks.NewMockenvGetter(ctrl)
				m.EXPECT().GetEnvironment(testProject, testEnv).Return(&archer.Environment{
					Project:        testProject,
					Name:           testEnv,
					ManagerRoleARN: testManagerRoleARN,
				}, nil)
				return m
			},
			mockStackDescribers: func(ctrl *gomock.Controller) map[string]stackDescriber {
				m := mocks.NewMockstackDescriber(ctrl)
				describers := make(map[string]stackDescriber)
				m.EXPECT().DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
					StackName: aws.String(stack.NameForApp(testProject, testEnv, testApp)),
				}).Return(nil, errors.New("some error"))
				describers[testManagerRoleARN] = m
				return describers
			},

			wantedError: fmt.Errorf("describe resources for stack phonetool-test-jobs: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				store:           tc.mockStore(ctrl),
				stackDescribers: tc.mockStackDescribers(ctrl),
			}

			// WHEN
			actual, err := d.StackResources(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedResources, actual)
			}
		})
	}
}
