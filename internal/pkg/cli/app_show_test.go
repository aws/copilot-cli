// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showAppMocks struct {
	storeSvc       *mocks.Mockstore
	sel            *mocks.MockappSelector
	deployStore    *mocks.MockdeployedEnvironmentLister
	pipelineGetter *mocks.MockpipelineGetter
	pipelineLister *mocks.MockdeployedPipelineLister
	versionGetter  *mocks.MockversionGetter
}

func TestShowAppOpts_Validate(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inAppName  string
		setupMocks func(mocks showAppMocks)

		wantedError error
	}{
		"valid app name": {
			inAppName: "my-app",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: nil,
		},
		"invalid app name": {
			inAppName: "my-app",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("get application %s: %w", "my-app", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := showAppMocks{
				storeSvc: mockStoreReader,
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					name: tc.inAppName,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestShowAppOpts_Ask(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inApp string

		setupMocks func(mocks showAppMocks)

		wantedApp   string
		wantedError error
	}{
		"with all flags": {
			inApp: "my-app",

			setupMocks: func(m showAppMocks) {},

			wantedApp:   "my-app",
			wantedError: nil,
		},
		"prompt for all input": {
			inApp: "",

			setupMocks: func(m showAppMocks) {
				m.sel.EXPECT().Application(appShowNamePrompt, appShowNameHelpPrompt).Return("my-app", nil)
			},
			wantedApp:   "my-app",
			wantedError: nil,
		},
		"returns error if failed to select application": {
			inApp: "",

			setupMocks: func(m showAppMocks) {
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", testError)
			},

			wantedError: fmt.Errorf("select application: %w", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := showAppMocks{
				sel: mocks.NewMockappSelector(ctrl),
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					name: tc.inApp,
				},
				sel: mocks.sel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, opts.name, "expected app names to match")

			}
		})
	}
}

func TestShowAppOpts_Execute(t *testing.T) {
	const (
		mockAppName            = "my-app"
		mockPipelineName       = "my-pipeline-repo"
		mockLegacyPipelineName = "bad-goose"
		mockTemplateVersion    = "v1.29.0"
	)
	mockPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: fmt.Sprintf("pipeline-%s-%s", mockAppName, mockPipelineName),
		Name:         mockPipelineName,
		IsLegacy:     false,
	}
	mockLegacyPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: mockLegacyPipelineName,
		IsLegacy:     true,
	}
	testError := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputJSON bool

		setupMocks func(mocks showAppMocks)

		wantedContent string
		wantedError   error
	}{
		"correctly shows json output": {
			shouldOutputJSON: true,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil).AnyTimes()
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("pipeline-my-app-my-pipeline-repo").Return(&codepipeline.Pipeline{
					Name: "my-pipeline-repo",
				}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("bad-goose").Return(&codepipeline.Pipeline{
					Name: "bad-goose",
				}, nil)
				m.versionGetter.EXPECT().Version().Return("v0.0.0", nil)
			},

			wantedContent: "{\"name\":\"my-app\",\"version\":\"v0.0.0\",\"uri\":\"example.com\",\"permissionsBoundary\":\"examplePermissionsBoundaryPolicy\",\"environments\":[{\"app\":\"\",\"name\":\"test\",\"region\":\"us-west-2\",\"accountID\":\"123456789\",\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},{\"app\":\"\",\"name\":\"prod\",\"region\":\"us-west-1\",\"accountID\":\"123456789\",\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"}],\"services\":[{\"app\":\"\",\"name\":\"my-svc\",\"type\":\"lb-web-svc\"}],\"jobs\":[{\"app\":\"\",\"name\":\"my-job\",\"type\":\"Scheduled Job\"}],\"pipelines\":[{\"pipelineName\":\"my-pipeline-repo\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"},{\"pipelineName\":\"bad-goose\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"}]}\n",
		},
		"correctly shows human output": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("pipeline-my-app-my-pipeline-repo").Return(&codepipeline.Pipeline{
					Name: "my-pipeline-repo",
				}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("bad-goose").Return(&codepipeline.Pipeline{
					Name: "bad-goose",
				}, nil)
				m.versionGetter.EXPECT().Version().Return("v0.0.0", nil)
			},

			wantedContent: `About

  Name                  my-app
  Version               v0.0.0
  URI                   example.com
  Permissions Boundary  examplePermissionsBoundaryPolicy

Environments

  Name    AccountID  Region
  ----    ---------  ------
  test    123456789  us-west-2
  prod    123456789  us-west-1

Workloads

  Name    Type           Environments
  ----    ----           ------------
  my-svc  lb-web-svc     test
  my-job  Scheduled Job  prod, test

Pipelines

  Name
  ----
  my-pipeline-repo
  bad-goose
`,
		},
		"correctly shows human output with latest version": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
			},

			wantedContent: `About

  Name                  my-app
  Version               v1.29.0
  URI                   example.com
  Permissions Boundary  examplePermissionsBoundaryPolicy

Environments

  Name    AccountID  Region
  ----    ---------  ------
  test    123456789  us-west-2
  prod    123456789  us-west-1

Workloads

  Name    Type           Environments
  ----    ----           ------------
  my-svc  lb-web-svc     prod, test
  my-job  Scheduled Job  prod, test

Pipelines

  Name
  ----
`,
		},
		"correctly shows human output when URI and Permissions Boundary are empty": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "",
					PermissionsBoundary: "",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)

			},

			wantedContent: `About

  Name                  my-app
  Version               v1.29.0
  URI                   N/A
  Permissions Boundary  N/A

Environments

  Name    AccountID  Region
  ----    ---------  ------
  test    123456789  us-west-2
  prod    123456789  us-west-1

Workloads

  Name    Type           Environments
  ----    ----           ------------
  my-svc  lb-web-svc     prod, test
  my-job  Scheduled Job  prod, test

Pipelines

  Name
  ----
`,
		},
		"when service/job is not deployed": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("pipeline-my-app-my-pipeline-repo").Return(&codepipeline.Pipeline{
					Name: "my-pipeline-repo",
				}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
			},

			wantedContent: `About

  Name                  my-app
  Version               v1.29.0
  URI                   example.com
  Permissions Boundary  examplePermissionsBoundaryPolicy

Environments

  Name    AccountID  Region
  ----    ---------  ------
  test    123456789  us-west-2
  prod    123456789  us-west-1

Workloads

  Name    Type           Environments
  ----    ----           ------------
  my-svc  lb-web-svc     -
  my-job  Scheduled Job  -

Pipelines

  Name
  ----
  my-pipeline-repo
`,
		},
		"when multiple services/jobs are deployed": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test1",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod1",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
					{
						Name:      "test2",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod2",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
					{
						Name:      "staging",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test1").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod1").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod2").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test2").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "staging").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test1").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod1").Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod2").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test2").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "staging").Return([]string{"my-svc"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("pipeline-my-app-my-pipeline-repo").Return(&codepipeline.Pipeline{
					Name: "my-pipeline-repo",
				}, nil)
				m.versionGetter.EXPECT().Version().Return(mockTemplateVersion, nil)
			},

			wantedContent: `About

  Name                  my-app
  Version               v1.29.0
  URI                   example.com
  Permissions Boundary  examplePermissionsBoundaryPolicy

Environments

  Name     AccountID  Region
  ----     ---------  ------
  test1    123456789  us-west-2
  prod1    123456789  us-west-1
  test2    123456789  us-west-2
  prod2    123456789  us-west-1
  staging  123456789  us-west-1

Workloads

  Name    Type           Environments
  ----    ----           ------------
  my-svc  lb-web-svc     prod2, staging, test2
  my-job  Scheduled Job  prod1, staging, test1

Pipelines

  Name
  ----
  my-pipeline-repo
`,
		},
		"returns error if fail to get application": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("get application %s: %w", "my-app", testError),
		},
		"returns error if fail to list environment": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("list environments in application %s: %w", "my-app", testError),
		},
		"returns error if fail to list services": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("list services in application %s: %w", "my-app", testError),
		},
		"returns error if fail to list jobs": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("list jobs in application %s: %w", "my-app", testError),
		},
		"returns error if fail to list pipelines": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("list pipelines in application %s: %w", "my-app", testError),
		},
		"returns error if fail to get pipeline info": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil)
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline("pipeline-my-app-my-pipeline-repo").Return(nil, testError)
			},
			wantedError: fmt.Errorf("get info for pipeline %s: %w", mockPipelineName, testError),
		},
		"returns error if fail to get app version": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:                "my-app",
					Domain:              "example.com",
					PermissionsBoundary: "examplePermissionsBoundaryPolicy",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Workload{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListJobs("my-app").Return([]*config.Workload{
					{
						Name: "my-job",
						Type: "Scheduled Job",
					},
				}, nil)
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "test").Return([]string{"my-job"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedJobs("my-app", "prod").Return([]string{"my-job"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedServices("my-app", "test").Return([]string{"my-svc"}, nil).AnyTimes()
				m.deployStore.EXPECT().ListDeployedServices("my-app", "prod").Return([]string{"my-svc"}, nil).AnyTimes()
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.versionGetter.EXPECT().Version().Return("", testError)
			},
			wantedError: fmt.Errorf("get version for application %s: %w", "my-app", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPLSvc := mocks.NewMockpipelineGetter(ctrl)
			mockVersionGetter := mocks.NewMockversionGetter(ctrl)
			mockPipelineLister := mocks.NewMockdeployedPipelineLister(ctrl)
			mockDeployStore := mocks.NewMockdeployedEnvironmentLister(ctrl)

			mocks := showAppMocks{
				storeSvc:       mockStoreReader,
				pipelineGetter: mockPLSvc,
				versionGetter:  mockVersionGetter,
				pipelineLister: mockPipelineLister,
				deployStore:    mockDeployStore,
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					name:             mockAppName,
				},
				store:          mockStoreReader,
				w:              b,
				codepipeline:   mockPLSvc,
				pipelineLister: mockPipelineLister,
				deployStore:    mockDeployStore,
				newVersionGetter: func(s string) (versionGetter, error) {
					return mockVersionGetter, nil
				},
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
