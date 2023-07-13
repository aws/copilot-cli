// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestServiceStackDescriber_Manifest(t *testing.T) {
	testApp, testEnv, testWorkload := "phonetool", "test", "api"
	testCases := map[string]struct {
		mockCFN func(ctrl *gomock.Controller) *mocks.MockstackDescriber

		wantedMft []byte
		wantedErr error
	}{
		"should return wrapped error if Metadata cannot be retrieved from stack": {
			mockCFN: func(ctrl *gomock.Controller) *mocks.MockstackDescriber {
				cfn := mocks.NewMockstackDescriber(ctrl)
				cfn.EXPECT().StackMetadata().Return("", errors.New("some error"))
				return cfn
			},
			wantedErr: errors.New("retrieve stack metadata for phonetool-test-api: some error"),
		},
		"should return ErrManifestNotFoundInTemplate if Metadata.Manifest is empty": {
			mockCFN: func(ctrl *gomock.Controller) *mocks.MockstackDescriber {
				cfn := mocks.NewMockstackDescriber(ctrl)
				cfn.EXPECT().StackMetadata().Return("", nil)
				return cfn
			},
			wantedErr: &ErrManifestNotFoundInTemplate{app: testApp, env: testEnv, name: testWorkload},
		},
		"should return content of Metadata.Manifest if it exists": {
			mockCFN: func(ctrl *gomock.Controller) *mocks.MockstackDescriber {
				cfn := mocks.NewMockstackDescriber(ctrl)
				cfn.EXPECT().StackMetadata().Return(`
Manifest: |
  hello`, nil)
				return cfn
			},
			wantedMft: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			describer := WorkloadStackDescriber{
				app:  testApp,
				env:  testEnv,
				name: testWorkload,
				cfn:  tc.mockCFN(ctrl),
			}

			// WHEN
			actualMft, actualErr := describer.Manifest()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error())
			} else {
				require.NoError(t, actualErr)
				require.Equal(t, tc.wantedMft, actualMft)
			}
		})
	}
}

func Test_WorkloadManifest(t *testing.T) {
	testApp, testService := "phonetool", "api"

	testCases := map[string]struct {
		inEnv         string
		mockDescriber func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) }

		wantedMft []byte
		wantedErr error
	}{
		"should return the error as is from the mock ecs client for LBWSDescriber": {
			inEnv: "test",
			mockDescriber: func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) } {
				m := mocks.NewMockecsDescriber(ctrl)
				m.EXPECT().Manifest().Return(nil, errors.New("some error"))
				return &LBWebServiceDescriber{
					app: testApp,
					svc: testService,
					initECSServiceDescribers: func(s string) (ecsDescriber, error) {
						return m, nil
					},
				}
			},
			wantedErr: errors.New("some error"),
		},
		"should return the error as is from the mock ecs client for BackendServiceDescriber": {
			inEnv: "test",
			mockDescriber: func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) } {
				m := mocks.NewMockecsDescriber(ctrl)
				m.EXPECT().Manifest().Return(nil, errors.New("some error"))
				return &BackendServiceDescriber{
					app: testApp,
					svc: testService,
					initECSServiceDescribers: func(s string) (ecsDescriber, error) {
						return m, nil
					},
				}
			},
			wantedErr: errors.New("some error"),
		},
		"should return the error as is from the mock app runner client for RDWebServiceDescriber": {
			inEnv: "test",
			mockDescriber: func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) } {
				m := mocks.NewMockapprunnerDescriber(ctrl)
				m.EXPECT().Manifest().Return(nil, errors.New("some error"))
				return &RDWebServiceDescriber{
					app: testApp,
					svc: testService,
					initAppRunnerDescriber: func(s string) (apprunnerDescriber, error) {
						return m, nil
					},
				}
			},
			wantedErr: errors.New("some error"),
		},
		"should return the error as is from the mock app runner client for WorkerServiceDescriber": {
			inEnv: "test",
			mockDescriber: func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) } {
				m := mocks.NewMockecsDescriber(ctrl)
				m.EXPECT().Manifest().Return(nil, errors.New("some error"))
				return &WorkerServiceDescriber{
					app: testApp,
					svc: testService,
					initECSDescriber: func(s string) (ecsDescriber, error) {
						return m, nil
					},
				}
			},
			wantedErr: errors.New("some error"),
		},
		"should return the manifest content on success for LBWSDescriber": {
			inEnv: "test",
			mockDescriber: func(ctrl *gomock.Controller) interface{ Manifest(string) ([]byte, error) } {
				m := mocks.NewMockecsDescriber(ctrl)
				m.EXPECT().Manifest().Return([]byte("hello"), nil)
				return &LBWebServiceDescriber{
					app: testApp,
					svc: testService,
					initECSServiceDescribers: func(s string) (ecsDescriber, error) {
						return m, nil
					},
				}
			},
			wantedMft: []byte("hello"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			describer := tc.mockDescriber(ctrl)

			// WHEN
			actualMft, actualErr := describer.Manifest(tc.inEnv)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error())
			} else {
				require.NoError(t, actualErr)
				require.Equal(t, tc.wantedMft, actualMft)
			}
		})
	}
}

func TestServiceDescriber_StackResources(t *testing.T) {
	const (
		testApp  = "phonetool"
		testEnv  = "test"
		testWkld = "jobs"
	)
	testdeployedSvcResources := []*stack.Resource{
		{
			Type:       "AWS::EC2::SecurityGroup",
			PhysicalID: "sg-0758ed6b233743530",
		},
	}
	testCases := map[string]struct {
		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedResources []*stack.Resource
		wantedError     error
	}{
		"returns error when fail to describe stack resources": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockCFN.EXPECT().Resources().Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"ignores dummy stack resources": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.mockCFN.EXPECT().Resources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
						{
							Type:       "AWS::CloudFormation::WaitConditionHandle",
							PhysicalID: "https://cloudformation-waitcondition-us-west-2.s3-us-west-2.amazonaws.com/",
						},
						{
							Type:       "Custom::RulePriorityFunction",
							PhysicalID: "alb-rule-priority-HTTPRulePriorityAction",
						},
						{
							Type:       "AWS::CloudFormation::WaitCondition",
							PhysicalID: "arn:aws:cloudformation:us-west-2:1234567890",
						},
					}, nil),
				)
			},

			wantedResources: testdeployedSvcResources,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCFN := mocks.NewMockstackDescriber(ctrl)
			mocks := ecsSvcDescriberMocks{
				mockCFN: mockCFN,
			}

			tc.setupMocks(mocks)

			d := &WorkloadStackDescriber{
				app:  testApp,
				name: testWkld,
				env:  testEnv,
				cfn:  mockCFN,
			}

			// WHEN
			actual, err := d.StackResources()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantedResources, actual)
			}
		})
	}
}
