// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestPackageSvcOpts_Validate(t *testing.T) {
	var (
		mockWorkspace *mocks.MockwsWlDirReader
		mockStore     *mocks.Mockstore
	)

	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inSvcName string

		setupMocks func()

		wantedErrorS string
	}{
		"invalid workspace": {
			setupMocks: func() {
				mockWorkspace.EXPECT().ListServices().Times(0)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find an application attached to this workspace, please run `app init` first",
		},
		"error while fetching service": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ListServices().Return(nil, errors.New("some error"))
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list services in the workspace: some error",
		},
		"error when service not in workspace": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ListServices().Return([]string{"backend"}, nil)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "service 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inAppName: "phonetool",
			inEnvName: "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().ListServices().Times(0)
				mockStore.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{
					ApplicationName: "phonetool",
					EnvironmentName: "test",
				})
			},

			wantedErrorS: (&config.ErrNoSuchEnvironment{
				ApplicationName: "phonetool",
				EnvironmentName: "test",
			}).Error(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace = mocks.NewMockwsWlDirReader(ctrl)
			mockStore = mocks.NewMockstore(ctrl)

			tc.setupMocks()

			opts := &packageSvcOpts{
				packageSvcVars: packageSvcVars{
					name:    tc.inSvcName,
					envName: tc.inEnvName,
					appName: tc.inAppName,
				},
				ws:    mockWorkspace,
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS, "error %v does not match '%s'", err, tc.wantedErrorS)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPackageSvcOpts_Ask(t *testing.T) {
	const testAppName = "phonetool"
	testCases := map[string]struct {
		inSvcName string
		inEnvName string

		expectSelector func(m *mocks.MockwsSelector)

		wantedSvcName string
		wantedEnvName string
		wantedErrorS  string
	}{
		"prompt only for the service name": {
			inEnvName: "test",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(svcPackageSvcNamePrompt, "").Return("frontend", nil)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
		},
		"prompt only for the env name": {
			inSvcName: "frontend",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(svcPackageEnvNamePrompt, "", testAppName).Return("test", nil)
			},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
		},
		"don't prompt": {
			inSvcName: "frontend",
			inEnvName: "test",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockwsSelector(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)

			tc.expectSelector(mockSelector)

			opts := &packageSvcOpts{
				packageSvcVars: packageSvcVars{
					name:    tc.inSvcName,
					envName: tc.inEnvName,
					appName: testAppName,
				},
				sel:    mockSelector,
				runner: mockRunner,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedSvcName, opts.name)
			require.Equal(t, tc.wantedEnvName, opts.envName)

			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPackageSvcOpts_Execute(t *testing.T) {
	const (
		mockARN    = "mockARN"
		mockDigest = "mockDigest"
		lbwsMft    = `name: api
type: Load Balanced Web Service
image:
  build: ./Dockerfile
  port: 80
http:
  path: 'api'
cpu: 256
memory: 512
count: 1`
		rdwsMft = `name: api
type: Request-Driven Web Service
image:
  build: ./Dockerfile
  port: 80
http:
  alias: 'hunter.com'
cpu: 256
memory: 512
count: 1`
	)
	testCases := map[string]struct {
		inVars packageSvcVars

		mockDependencies func(*gomock.Controller, *packageSvcOpts)

		wantedStack  string
		wantedParams string
		wantedAddons string
		wantedErr    error
	}{
		"writes service template without addons": {
			inVars: packageSvcVars{
				appName:          "ecs-kudos",
				name:             "api",
				envName:          "test",
				tag:              "1234",
				clientConfigured: true,
				uploadAssets:     true,
			},
			mockDependencies: func(ctrl *gomock.Controller, opts *packageSvcOpts) {
				mockWs := mocks.NewMockwsWlDirReader(ctrl)
				mockWs.EXPECT().
					ReadWorkloadManifest("api").
					Return([]byte(lbwsMft), nil)

				mockGenerator := mocks.NewMockworkloadTemplateGenerator(ctrl)
				mockGenerator.EXPECT().UploadArtifacts().Return(&deploy.UploadArtifactsOutput{
					ImageDigest: aws.String(mockDigest),
				}, nil)
				mockGenerator.EXPECT().GenerateCloudFormationTemplate(&deploy.GenerateCloudFormationTemplateInput{
					StackRuntimeConfiguration: deploy.StackRuntimeConfiguration{
						ImageDigest: aws.String(mockDigest),
						RootUserARN: mockARN,
					},
				}).
					Return(&deploy.GenerateCloudFormationTemplateOutput{
						Template:   "mystack",
						Parameters: "myparams",
					}, nil)

				mockItpl := mocks.NewMockinterpolator(ctrl)
				mockItpl.EXPECT().Interpolate(lbwsMft).Return(lbwsMft, nil)

				mockAddons := mocks.NewMocktemplater(ctrl)
				mockAddons.EXPECT().Template().
					Return("", &addon.ErrAddonsNotFound{})

				opts.ws = mockWs
				opts.initAddonsClient = func(opts *packageSvcOpts) error {
					opts.addonsClient = mockAddons
					return nil
				}
				opts.newInterpolator = func(app, env string) interpolator {
					return mockItpl
				}
				opts.newTplGenerator = func(pso *packageSvcOpts) (workloadTemplateGenerator, error) {
					return mockGenerator, nil
				}
			},

			wantedStack:  "mystack",
			wantedParams: "myparams",
		},
		"writes request-driven web service template with custom resource": {
			inVars: packageSvcVars{
				appName:          "ecs-kudos",
				name:             "api",
				envName:          "test",
				tag:              "1234",
				clientConfigured: true,
			},
			mockDependencies: func(ctrl *gomock.Controller, opts *packageSvcOpts) {
				mockWs := mocks.NewMockwsWlDirReader(ctrl)
				mockWs.EXPECT().
					ReadWorkloadManifest("api").
					Return([]byte(rdwsMft), nil)

				mockItpl := mocks.NewMockinterpolator(ctrl)
				mockItpl.EXPECT().Interpolate(rdwsMft).Return(rdwsMft, nil)

				mockAddons := mocks.NewMocktemplater(ctrl)
				mockAddons.EXPECT().Template().
					Return("", &addon.ErrAddonsNotFound{})

				mockGenerator := mocks.NewMockworkloadTemplateGenerator(ctrl)
				mockGenerator.EXPECT().GenerateCloudFormationTemplate(&deploy.GenerateCloudFormationTemplateInput{
					StackRuntimeConfiguration: deploy.StackRuntimeConfiguration{
						ImageDigest: aws.String(""),
						RootUserARN: mockARN,
					},
				}).
					Return(&deploy.GenerateCloudFormationTemplateOutput{
						Template:   "mystack",
						Parameters: "myparams",
					}, nil)

				opts.ws = mockWs
				opts.initAddonsClient = func(opts *packageSvcOpts) error {
					opts.addonsClient = mockAddons
					return nil
				}
				opts.newInterpolator = func(app, env string) interpolator {
					return mockItpl
				}
				opts.newTplGenerator = func(pso *packageSvcOpts) (workloadTemplateGenerator, error) {
					return mockGenerator, nil
				}
			},

			wantedStack:  "mystack",
			wantedParams: "myparams",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stackBuf := new(bytes.Buffer)
			paramsBuf := new(bytes.Buffer)
			addonsBuf := new(bytes.Buffer)
			opts := &packageSvcOpts{
				packageSvcVars: tc.inVars,

				stackWriter:  stackBuf,
				paramsWriter: paramsBuf,
				addonsWriter: addonsBuf,
				unmarshal: func(b []byte) (manifest.WorkloadManifest, error) {
					return &mockWorkloadMft{}, nil
				},
				rootUserARN: mockARN,
				targetApp:   &config.Application{},
			}
			tc.mockDependencies(ctrl, opts)

			// WHEN
			err := opts.Execute()

			// THEN
			require.Equal(t, tc.wantedErr, err)
			require.Equal(t, tc.wantedStack, stackBuf.String())
			require.Equal(t, tc.wantedParams, paramsBuf.String())
			require.Equal(t, tc.wantedAddons, addonsBuf.String())
		})
	}
}
