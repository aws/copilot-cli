// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package patch

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy/patch/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type envPatcherMock struct {
	templatePatcher *mocks.MockenvironmentTemplateUpdateGetter
	prog            *mocks.Mockprogress
}

func TestEnvironmentPatcher_EnsureManagerRoleIsAllowedToUpload(t *testing.T) {
	testCases := map[string]struct {
		setupMocks  func(m *envPatcherMock)
		wantedError error
	}{
		"error getting environment template": {
			setupMocks: func(m *envPatcherMock) {
				m.templatePatcher.EXPECT().Template(stack.NameForEnv("mockApp", "mockEnv")).
					Return("", errors.New("some error"))
			},
			wantedError: errors.New(`get environment template for "mockEnv": some error`),
		},
		"error updating the environment template with the patch": {
			setupMocks: func(m *envPatcherMock) {
				m.templatePatcher.EXPECT().Template(stack.NameForEnv("mockApp", "mockEnv")).
					Return(`
Metadata:
  Version: v1.7.0
Resources:
  EnvironmentManagerRole:
    Properties:
      Policies:
        - PolicyDocument:
            Statement:
            - Sid: CloudwatchLogs 
  OtherResource:`, nil)
				m.prog.EXPECT().Start(gomock.Any())
				m.templatePatcher.EXPECT().UpdateEnvironmentTemplate("mockApp", "mockEnv", `
Metadata:
  Version: v1.7.0
Resources:
  EnvironmentManagerRole:
    Properties:
      Policies:
        - PolicyDocument:
            Statement:
            - Sid: PatchPutObjectsToArtifactBucket
              Effect: Allow
              Action:
                - s3:PutObject
                - s3:PutObjectAcl
              Resource:
                - arn:aws:s3:::mockBucket
                - arn:aws:s3:::mockBucket/*
            - Sid: CloudwatchLogs 
  OtherResource:`, "mockExecutionRoleARN").Return(errors.New("some error"))
				m.prog.EXPECT().Stop(gomock.Any())
			},
			wantedError: errors.New("update environment template with PutObject permissions: some error"),
		},
		"success when template version is later than v1.9.0": {
			setupMocks: func(m *envPatcherMock) {
				m.templatePatcher.EXPECT().Template(stack.NameForEnv("mockApp", "mockEnv")).
					Return(`
Metadata:
  Version: v1.9.0`, nil)
			},
		},
		"should upgrade non-legacy environments and ignore ErrChangeSet if the environment template already has permissions to upload artifacts": {
			setupMocks: func(m *envPatcherMock) {
				m.templatePatcher.EXPECT().Template(stack.NameForEnv("mockApp", "mockEnv")).Return(`
Metadata:
  Version: v1.1.0
Resources:
  EnvironmentManagerRole:
    Properties:
      Policies:
        - PolicyDocument:
            Statement:
            - Sid: PatchPutObjectsToArtifactBucket
              Effect: Allow
              Action:
                - s3:PutObject
                - s3:PutObjectAcl
              Resource:
                - arn:aws:s3:::mockBucket
                - arn:aws:s3:::mockBucket/*
`, nil)
				m.prog.EXPECT().Start(gomock.Any())
				m.templatePatcher.EXPECT().UpdateEnvironmentTemplate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("wrapped err: %w", &cloudformation.ErrChangeSetEmpty{}))
				m.prog.EXPECT().Stop(gomock.Any())
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &envPatcherMock{
				templatePatcher: mocks.NewMockenvironmentTemplateUpdateGetter(ctrl),
				prog:            mocks.NewMockprogress(ctrl),
			}
			tc.setupMocks(m)
			p := EnvironmentPatcher{
				Env: &config.Environment{
					App:              "mockApp",
					Name:             "mockEnv",
					ExecutionRoleARN: "mockExecutionRoleARN",
				},
				TemplatePatcher: m.templatePatcher,
				Prog:            m.prog,
			}

			got := p.EnsureManagerRoleIsAllowedToUpload("mockBucket")
			if tc.wantedError != nil {
				require.EqualError(t, got, tc.wantedError.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}
