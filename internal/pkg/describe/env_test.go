// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvDescriber_JSONString(t *testing.T) {
	mockApplications := []*archer.Application{
		{Project: "my-project",
			Name: "my-app",
			Type: "lb-web-app"},
		{Project: "my-project",
			Name: "copilot-app",
			Type: "lb-web-app"},
	}
	mockProject := &archer.Project{
		Name: "my-project",
		Tags: map[string]string{"tag1": "value1", "tag2": "value2"},
	}
	mockEnv := &archer.Environment{
		Project:          "my-project",
		Name:             "test",
		Region:           "us-west-2",
		AccountID:        "123456789",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}

	testCases := map[string]struct {
		shouldOutputJSON bool
		wantedError      error
		wantedContent    string
	}{
		"correctly shows json output": {
			shouldOutputJSON: true,
			wantedContent:    "{\"environment\":{\"project\":\"my-project\",\"name\":\"test\",\"region\":\"us-west-2\",\"accountID\":\"123456789\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},\"applications\":[{\"project\":\"my-project\",\"name\":\"my-app\",\"type\":\"lb-web-app\"},{\"project\":\"my-project\",\"name\":\"copilot-app\",\"type\":\"lb-web-app\"}],\"tags\":{\"tag1\":\"value1\",\"tag2\":\"value2\"}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &EnvDescriber{
				env:  mockEnv,
				proj: mockProject,
				apps: mockApplications,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, actual)
			}
		})
	}
}

func TestEnvDescriber_HumanString(t *testing.T) {
	mockApplications := []*archer.Application{
		{Project: "my-project",
			Name: "my-app",
			Type: "lb-web-app"},
		{Project: "my-project",
			Name: "copilot-app",
			Type: "lb-web-app"},
	}
	mockProject := &archer.Project{
		Name: "my-project",
		Tags: map[string]string{"tag1": "value1", "tag2": "value2"},
	}
	mockEnv := &archer.Environment{
		Project:          "my-project",
		Name:             "test",
		Region:           "us-west-2",
		AccountID:        "123456789",
		Prod:             false,
		RegistryURL:      "",
		ExecutionRoleARN: "",
		ManagerRoleARN:   "",
	}

	testCases := map[string]struct {
		shouldOutputJSON bool
		wantedError      error
		wantedContent    string
	}{
		"correctly shows human output": {
			shouldOutputJSON: false,
			wantedContent: `About

  Name              test
  Production        false
  Region            us-west-2
  Account ID        123456789

Applications

  Name              Type
  my-app            lb-web-app
  copilot-app       lb-web-app

Tags

  Key               Value
  tag1              value1
  tag2              value2
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			d := &EnvDescriber{
				env:  mockEnv,
				proj: mockProject,
				apps: mockApplications,
			}

			// WHEN
			actual, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, actual)
			}
		})
	}
}
