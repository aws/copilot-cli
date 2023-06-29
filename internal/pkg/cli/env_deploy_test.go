// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type deployEnvAskMocks struct {
	ws    *mocks.MockwsEnvironmentReader
	sel   *mocks.MockwsEnvironmentSelector
	store *mocks.Mockstore
}

func TestDeployEnvOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName  string
		inName     string
		setUpMocks func(m *deployEnvAskMocks)

		wantedEnvName string
		wantedError   error
	}{
		"fail to retrieve app from store when validating app": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get application mockApp: some error"),
		},
		"error if no app in workspace": {
			inName: "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Times(0)
			},
			wantedError: errNoAppInWorkspace,
		},
		"fail to list environments in local workspace": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("list environments in workspace: some error"),
		},
		"fail to find local environment manifest workspace": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"otherEnv"}, nil)
			},
			wantedError: errors.New(`environment manifest for "mockEnv" is not found`),
		},
		"fail to retrieve env from store when validating env": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("get environment mockEnv in application mockApp: some error"),
		},
		"fail to ask for an env from workspace": {
			inAppName: "mockApp",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").AnyTimes()
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedError: errors.New("select environment: some error"),
		},
		"validate env instead of asking if it is passed in as a flag": {
			inAppName: "mockApp",
			inName:    "mockEnv",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.ws.EXPECT().ListEnvironments().Return([]string{"mockEnv"}, nil)
				m.store.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{}, nil)
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedEnvName: "mockEnv",
		},
		"ask for env": {
			inAppName: "mockApp",
			setUpMocks: func(m *deployEnvAskMocks) {
				m.store.EXPECT().GetApplication("mockApp").Return(&config.Application{}, nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.ws.EXPECT().ListEnvironments().Times(0)
				m.sel.EXPECT().LocalEnvironment(gomock.Any(), gomock.Any()).Return("mockEnv", nil)
			},
			wantedEnvName: "mockEnv",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &deployEnvAskMocks{
				ws:    mocks.NewMockwsEnvironmentReader(ctrl),
				sel:   mocks.NewMockwsEnvironmentSelector(ctrl),
				store: mocks.NewMockstore(ctrl),
			}
			tc.setUpMocks(m)
			opts := deployEnvOpts{
				deployEnvVars: deployEnvVars{
					appName: tc.inAppName,
					name:    tc.inName,
				},
				ws:    m.ws,
				sel:   m.sel,
				store: m.store,
			}
			gotErr := opts.Ask()
			if tc.wantedError != nil {
				require.EqualError(t, gotErr, tc.wantedError.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, opts.name, tc.wantedEnvName)
			}
		})
	}
}

type deployEnvExecuteMocks struct {
	ws               *mocks.MockwsEnvironmentReader
	deployer         *mocks.MockenvDeployer
	identity         *mocks.MockidentityService
	interpolator     *mocks.Mockinterpolator
	prompter         *mocks.Mockprompter
	envVersionGetter *mocks.MockversionGetter
}

func TestDeployEnvOpts_Execute(t *testing.T) {
	const (
		mockEnvVersion       = "v0.0.0"
		mockCurrVersion      = "v1.29.0"
		mockFutureEnvVersion = "v2.0.0"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		inShowDiff        bool
		inSkipDiffPrompt  bool
		inAllowDowngrade  bool
		unmarshalManifest func(in []byte) (*manifest.Environment, error)
		setUpMocks        func(m *deployEnvExecuteMocks)
		wantedDiff        string
		wantedErr         error
	}{
		"fail to get env version": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return("", mockError)
			},
			wantedErr: errors.New(`get template version of environment mockEnv: some error`),
		},
		"error for downgrading": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockFutureEnvVersion, nil)
			},
			wantedErr: errors.New(`cannot downgrade environment "mockEnv" (currently in version v2.0.0) to version v1.29.0`),
		},
		"fail to read manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return(nil, mockError)
			},
			wantedErr: errors.New(`read manifest for environment "mockEnv": some error`),
		},
		"fail to interpolate manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New(`interpolate environment variables for "mockEnv" manifest: some error`),
		},
		"fail to unmarshal manifest": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("failing manifest format", nil)
			},
			wantedErr: errors.New(`unmarshal environment manifest for "mockEnv"`),
		},
		"fail to get caller identity": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{}, errors.New("some error"))
			},
			wantedErr: errors.New("get identity: some error"),
		},
		"fail to verify env": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\ncdn: true\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\ncdn: true\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(errors.New("mock error"))
			},
			wantedErr: errors.New("mock error"),
		},
		"fail to upload artifacts": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("upload artifacts for environment mockEnv: some error"),
		},
		"fail to generate the template to show diff": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: fmt.Errorf(`generate the template for environment "mockEnv": some error`),
		},
		"failed to generate the diff": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New(`generate diff for environment "mockEnv": some error`),
		},
		"write 'no changes' if there is no diff": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "No changes.\n",
		},
		"write the correct diff": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDiff: "mock diff",
		},
		"error if fail to ask whether to continue the deployment": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			wantedErr: errors.New("ask whether to continue with the deployment: some error"),
		},
		"do not deploy if asked to": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Return(false, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).Times(0)
			},
		},
		"deploy if asked to": {
			inShowDiff: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Return(true, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).Times(1)
			},
		},
		"skip prompt and deploy immediately after diff; also skip version check when downgrade is allowed": {
			inShowDiff:       true,
			inSkipDiffPrompt: true,
			inAllowDowngrade: true,
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Times(0)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().GenerateCloudFormationTemplate(gomock.Any()).Return(&deploy.GenerateCloudFormationTemplateOutput{}, nil)
				m.deployer.EXPECT().DeployDiff(gomock.Any()).Return("", nil)
				m.prompter.EXPECT().Confirm(gomock.Eq(continueDeploymentPrompt), gomock.Any(), gomock.Any()).Times(0)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).Times(1)
			},
		},
		"fail to deploy the environment": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(mockEnvVersion, nil)
				m.ws.EXPECT().ReadEnvironmentManifest(gomock.Any()).Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate(gomock.Any()).Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{}, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).DoAndReturn(func(_ *deploy.DeployEnvironmentInput) error {
					return errors.New("some error")
				})
			},
			wantedErr: errors.New("deploy environment mockEnv: some error"),
		},
		"success": {
			setUpMocks: func(m *deployEnvExecuteMocks) {
				m.envVersionGetter.EXPECT().Version().Return(version.EnvTemplateBootstrap, nil)
				m.ws.EXPECT().ReadEnvironmentManifest("mockEnv").Return([]byte("name: mockEnv\ntype: Environment\n"), nil)
				m.interpolator.EXPECT().Interpolate("name: mockEnv\ntype: Environment\n").Return("name: mockEnv\ntype: Environment\n", nil)
				m.identity.EXPECT().Get().Return(identity.Caller{
					RootUserARN: "mockRootUserARN",
				}, nil)
				m.deployer.EXPECT().Validate(gomock.Any()).Return(nil)
				m.deployer.EXPECT().UploadArtifacts().Return(&deploy.UploadEnvArtifactsOutput{
					AddonsURL: "mockAddonsURL",
					CustomResourceURLs: map[string]string{
						"mockResource": "mockURL",
					},
				}, nil)
				m.deployer.EXPECT().DeployEnvironment(gomock.Any()).DoAndReturn(func(in *deploy.DeployEnvironmentInput) error {
					require.Equal(t, in.RootUserARN, "mockRootUserARN")
					require.Equal(t, in.AddonsURL, "mockAddonsURL")
					require.Equal(t, in.CustomResourcesURLs, map[string]string{
						"mockResource": "mockURL",
					})
					require.Equal(t, in.Manifest, &manifest.Environment{
						Workload: manifest.Workload{
							Name: aws.String("mockEnv"),
							Type: aws.String("Environment"),
						},
					})
					return nil
				})
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &deployEnvExecuteMocks{
				ws:               mocks.NewMockwsEnvironmentReader(ctrl),
				deployer:         mocks.NewMockenvDeployer(ctrl),
				identity:         mocks.NewMockidentityService(ctrl),
				interpolator:     mocks.NewMockinterpolator(ctrl),
				prompter:         mocks.NewMockprompter(ctrl),
				envVersionGetter: mocks.NewMockversionGetter(ctrl),
			}
			tc.setUpMocks(m)
			opts := deployEnvOpts{
				deployEnvVars: deployEnvVars{
					name:              "mockEnv",
					showDiff:          tc.inShowDiff,
					skipDiffPrompt:    tc.inSkipDiffPrompt,
					allowEnvDowngrade: tc.inAllowDowngrade,
				},
				ws:       m.ws,
				identity: m.identity,
				newEnvDeployer: func() (envDeployer, error) {
					return m.deployer, nil
				},
				newEnvVersionGetter: func(appName, envName string) (versionGetter, error) {
					return m.envVersionGetter, nil
				},
				templateVersion: mockCurrVersion,
				newInterpolator: func(s string, s2 string) interpolator {
					return m.interpolator
				},
				prompt: m.prompter,
				targetApp: &config.Application{
					Name:   "mockApp",
					Domain: "mockDomain",
				},
				targetEnv: &config.Environment{
					Name: "mockEnv",
				},
			}
			err := opts.Execute()
			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
