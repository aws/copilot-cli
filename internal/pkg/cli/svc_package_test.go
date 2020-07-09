// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestPackageSvcOpts_Validate(t *testing.T) {
	var (
		mockWorkspace *mocks.MockwsSvcReader
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
				mockWorkspace.EXPECT().ServiceNames().Times(0)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find an application attached to this workspace, please run `app init` first",
		},
		"error while fetching service": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Return(nil, errors.New("some error"))
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list services in the workspace: some error",
		},
		"error when service not in workspace": {
			inAppName: "phonetool",
			inSvcName: "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Return([]string{"backend"}, nil)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "service 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inAppName: "phonetool",
			inEnvName: "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Times(0)
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

			mockWorkspace = mocks.NewMockwsSvcReader(ctrl)
			mockStore = mocks.NewMockstore(ctrl)

			tc.setupMocks()

			opts := &packageSvcOpts{
				packageSvcVars: packageSvcVars{
					Name:       tc.inSvcName,
					EnvName:    tc.inEnvName,
					GlobalOpts: &GlobalOpts{appName: tc.inAppName},
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
				require.Nil(t, err)
			}
		})
	}
}

func TestPackageSvcOpts_Ask(t *testing.T) {
	const testAppName = "phonetool"
	testCases := map[string]struct {
		inSvcName string
		inEnvName string
		inTag     string

		expectSelector func(m *mocks.MockwsSelector)
		expectPrompt   func(m *mocks.Mockprompter)
		expectRunner   func(m *mocks.Mockrunner)

		wantedSvcName string
		wantedEnvName string
		wantedTag     string
		wantedErrorS  string
	}{
		"prompt for all options": {
			expectRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(svcPackageSvcNamePrompt, "").Return("frontend", nil)
				m.EXPECT().Environment(svcPackageEnvNamePrompt, "", testAppName).Return("test", nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(inputImageTagPrompt, "", nil).Return("v1.0.0", nil)
			},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the service name": {
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(svcPackageSvcNamePrompt, "").Return("frontend", nil)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the env name": {
			inSvcName: "frontend",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(svcPackageEnvNamePrompt, "", testAppName).Return("test", nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt": {
			inSvcName: "frontend",
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedSvcName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockwsSelector(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)

			tc.expectSelector(mockSelector)
			tc.expectPrompt(mockPrompt)
			tc.expectRunner(mockRunner)

			opts := &packageSvcOpts{
				packageSvcVars: packageSvcVars{
					Name:    tc.inSvcName,
					EnvName: tc.inEnvName,
					Tag:     tc.inTag,
					GlobalOpts: &GlobalOpts{
						appName: testAppName,
						prompt:  mockPrompt,
					},
				},
				sel:    mockSelector,
				runner: mockRunner,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedSvcName, opts.Name)
			require.Equal(t, tc.wantedEnvName, opts.EnvName)
			require.Equal(t, tc.wantedTag, opts.Tag)

			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestPackageSvcOpts_Execute(t *testing.T) {
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
				GlobalOpts: &GlobalOpts{
					appName: "ecs-kudos",
				},
				Name:    "api",
				EnvName: "test",
				Tag:     "1234",
			},
			mockDependencies: func(ctrl *gomock.Controller, opts *packageSvcOpts) {
				mockStore := mocks.NewMockstore(ctrl)
				mockStore.EXPECT().
					GetEnvironment("ecs-kudos", "test").
					Return(&config.Environment{
						App:       "ecs-kudos",
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "1111",
					}, nil)
				mockApp := &config.Application{
					Name:      "ecs-kudos",
					AccountID: "1112",
					Tags: map[string]string{
						"owner": "boss",
					},
				}
				mockStore.EXPECT().
					GetApplication("ecs-kudos").
					Return(mockApp, nil)

				mockWs := mocks.NewMockwsSvcReader(ctrl)
				mockWs.EXPECT().
					ReadServiceManifest("api").
					Return([]byte(`name: api
type: Load Balanced Web Service
image:
  build: ./Dockerfile
  port: 80
http:
  path: 'api'
cpu: 256
memory: 512
count: 1`), nil)

				mockCfn := mocks.NewMockappResourcesGetter(ctrl)
				mockCfn.EXPECT().
					GetAppResourcesByRegion(mockApp, "us-west-2").
					Return(&stack.AppRegionalResources{
						RepositoryURLs: map[string]string{
							"api": "some url",
						},
					}, nil)

				mockAddons := mocks.NewMocktemplater(ctrl)
				mockAddons.EXPECT().Template().
					Return("", &addon.ErrDirNotExist{})

				opts.store = mockStore
				opts.ws = mockWs
				opts.appCFN = mockCfn
				opts.initAddonsSvc = func(opts *packageSvcOpts) error {
					opts.addonsSvc = mockAddons
					return nil
				}
				opts.stackSerializer = func(_ interface{}, _ *config.Environment, _ *config.Application, _ stack.RuntimeConfig) (stackSerializer, error) {
					mockStackSerializer := mocks.NewMockstackSerializer(ctrl)
					mockStackSerializer.EXPECT().Template().Return("mystack", nil)
					mockStackSerializer.EXPECT().SerializedParameters().Return("myparams", nil)
					return mockStackSerializer, nil
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
