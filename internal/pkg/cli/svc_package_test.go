// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
)

func TestPackageSvcOpts_Validate(t *testing.T) {
	// NOTE: no optional flag needs to be validated for this command.
}

type svcPackageAskMock struct {
	store *mocks.Mockstore
	sel   *mocks.MockwsSelector
	ws    *mocks.MockwsWlDirReader
}

func TestPackageSvcOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inSvcName string
		inEnvName string

		setupMocks func(m svcPackageAskMock)

		wantedAppName string
		wantedSvcName string
		wantedEnvName string
		wantedError   error
	}{
		"validate instead of prompting application name, svc name and environment name": {
			inAppName: "phonetool",
			inEnvName: "prod-iad",
			inSvcName: "frontend",
			setupMocks: func(m svcPackageAskMock) {
				m.store.EXPECT().GetApplication("phonetool")
				m.store.EXPECT().GetEnvironment("phonetool", "prod-iad").Return(&config.Environment{Name: "prod-iad"}, nil)
				m.ws.EXPECT().ListServices().Return([]string{"frontend"}, nil)
				m.sel.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedAppName: "phonetool",
			wantedSvcName: "frontend",
			wantedEnvName: "prod-iad",
		},
		"error instead of prompting for application name if not provided": {
			setupMocks: func(m svcPackageAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
			},
			wantedError: errNoAppInWorkspace,
		},
		"prompt for the service name": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m svcPackageAskMock) {
				m.sel.EXPECT().Service("Which service would you like to generate a CloudFormation template for?", "").
					Return("frontend", nil)
				m.ws.EXPECT().ListServices().Times(0)
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedAppName: "phonetool",
			wantedSvcName: "frontend",
			wantedEnvName: "test",
		},
		"prompt for the env name": {
			inAppName: "phonetool",
			inSvcName: "frontend",

			setupMocks: func(m svcPackageAskMock) {
				m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), "phonetool").Return("prod-iad", nil)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetApplication("phonetool").AnyTimes()
				m.ws.EXPECT().ListServices().Return([]string{"frontend"}, nil).AnyTimes()
			},
			wantedAppName: "phonetool",
			wantedSvcName: "frontend",
			wantedEnvName: "prod-iad",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcPackageAskMock{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockwsSelector(ctrl),
				ws:    mocks.NewMockwsWlDirReader(ctrl),
			}
			tc.setupMocks(m)
			opts := &packageSvcOpts{
				packageSvcVars: packageSvcVars{
					name:    tc.inSvcName,
					envName: tc.inEnvName,
					appName: tc.inAppName,
				},
				sel:    m.sel,
				store:  m.store,
				ws:     m.ws,
				runner: mocks.NewMockrunner(ctrl),
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedAppName, opts.appName)
				require.Equal(t, tc.wantedSvcName, opts.name)
				require.Equal(t, tc.wantedEnvName, opts.envName)
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
				targetEnv:   &config.Environment{},
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
