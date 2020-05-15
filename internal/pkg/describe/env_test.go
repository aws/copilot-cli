// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envDescriberMocks struct {
	storeSvc                 *mocks.MockstoreSvc
	mockResourceGroupsClient *mocks.MockresourceGroupsClient
}

func TestEnvDescriber_Describe(t *testing.T) {
	const (
		testARN1      = "arn:aws:cloudformation:us-west-2:123456789012:stack/testApp-testEnv-testSvc1/6d75d1g0-8b1a-11ea-b358-06c1882c17fd"
		testARN2      = "arn:aws:cloudformation:us-west-2:123456789012:stack/testApp-testEnv-testSvc2/7d75d1f0-8c1a-11ea-b358-06c1882c17fc"
		unparsableARN = "aws:cloudformation:us-west-2:123456789012:stack/testApp-testEnv-testSvc2/7d75d1f0-8c1a-11ea-b358-06c1882c17fc"
		noSlashARN    = "arn:aws:cloudformation:us-west-2:123456789012:stacktestApp-testEnv-testSvc16d75d1g0-8b1a-11ea-b358-06c1882c17fd"
	)
	testApp := &config.Application{
		Name: "testApp",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Service{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Service{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Service{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}

	allSvcs := []*config.Service{testSvc1, testSvc2, testSvc3}
	envSvcs := []*config.Service{testSvc1, testSvc2}
	rgTags := map[string]string{stack.EnvTagKey: "testEnv"}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks envDescriberMocks)

		wantedEnv   *EnvDescription
		wantedSvcs  []*config.Service
		wantedError error
	}{
		"error if GetResourcesByTags fails": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).
						Return(nil, mockError),
				)
			},

			wantedError: fmt.Errorf("get AWS::CloudFormation::Stack resources for env testEnv: some error"),
		},
		"error if getStackName fails because can't parse resource ARN": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).Return([]string{
						unparsableARN,
					}, nil),
				)
			},
			wantedError: fmt.Errorf("parse ARN aws:cloudformation:us-west-2:123456789012:stack/testApp-testEnv-testSvc2/7d75d1f0-8c1a-11ea-b358-06c1882c17fc: arn: invalid prefix"),
		},
		"error if getStackName fails because resource ARN can't be split": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).Return([]string{
						noSlashARN,
					}, nil),
				)
			},
			wantedError: fmt.Errorf("invalid ARN resource format stacktestApp-testEnv-testSvc16d75d1g0-8b1a-11ea-b358-06c1882c17fd. Ex: arn:partition:service:region:account-id:resource-type/resource-id"),
		},
		"success": {
			setupMocks: func(m envDescriberMocks) {
				gomock.InOrder(
					m.mockResourceGroupsClient.EXPECT().GetResourcesByTags(cloudformationResourceType, rgTags).Return([]string{
						testARN1,
						testARN2,
					}, nil),
				)
			},
			wantedEnv: &EnvDescription{
				Environment: testEnv,
				Services:    envSvcs,
				Tags:        map[string]string{"key1": "value1", "key2": "value2"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstoreSvc(ctrl)
			mockResourceGroupsClient := mocks.NewMockresourceGroupsClient(ctrl)
			mocks := envDescriberMocks{
				storeSvc:                 mockStore,
				mockResourceGroupsClient: mockResourceGroupsClient,
			}

			tc.setupMocks(mocks)

			d := &EnvDescriber{
				env:  testEnv,
				app:  testApp,
				svcs: allSvcs,

				store:    mockStore,
				rgClient: mockResourceGroupsClient,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedEnv, actual)
			}
		})
	}
}

func TestEnvDescription_JSONString(t *testing.T) {
	testApp := &config.Application{
		Name: "testApp",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Service{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Service{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Service{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	allSvcs := []*config.Service{testSvc1, testSvc2, testSvc3}
	wantedContent := "{\"environment\":{\"app\":\"testApp\",\"name\":\"testEnv\",\"region\":\"us-west-2\",\"accountID\":\"123456789012\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"services\":[{\"app\":\"testApp\",\"name\":\"testSvc1\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc2\",\"type\":\"load-balanced\"},{\"app\":\"testApp\",\"name\":\"testSvc3\",\"type\":\"load-balanced\"}],\"tags\":{\"key1\":\"value1\",\"key2\":\"value2\"}}\n"

	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := &EnvDescription{
		Environment: testEnv,
		Services:    allSvcs,
		Tags:        testApp.Tags,
	}

	// WHEN
	actual, _ := d.JSONString()

	// THEN
	require.Equal(t, wantedContent, actual)
}

func TestEnvDescription_HumanString(t *testing.T) {
	testApp := &config.Application{
		Name: "testApp",
		Tags: map[string]string{"key1": "value1", "key2": "value2"},
	}
	testEnv := &config.Environment{
		App:              "testApp",
		Name:             "testEnv",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}
	testSvc1 := &config.Service{
		App:  "testApp",
		Name: "testSvc1",
		Type: "load-balanced",
	}
	testSvc2 := &config.Service{
		App:  "testApp",
		Name: "testSvc2",
		Type: "load-balanced",
	}
	testSvc3 := &config.Service{
		App:  "testApp",
		Name: "testSvc3",
		Type: "load-balanced",
	}
	allSvcs := []*config.Service{testSvc1, testSvc2, testSvc3}

	wantedContent := `About

  Name              testEnv
  Production        false
  Region            us-west-2
  Account ID        123456789012

Services

  Name              Type
  testSvc1          load-balanced
  testSvc2          load-balanced
  testSvc3          load-balanced

Tags

  Key               Value
  key1              value1
  key2              value2
`
	// GIVEN
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	d := &EnvDescription{
		Environment: testEnv,
		Services:    allSvcs,
		Tags:        testApp.Tags,
	}

	// WHEN
	actual := d.HumanString()

	// THEN
	require.Equal(t, wantedContent, actual)
}
