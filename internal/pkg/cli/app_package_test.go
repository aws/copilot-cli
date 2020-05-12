// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/addons"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestPackageAppOpts_Validate(t *testing.T) {
	var (
		mockWorkspace      *mocks.MockwsSvcReader
		mockProjectService *mocks.Mockstore
	)

	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inAppName     string

		setupMocks func()

		wantedErrorS string
	}{
		"invalid workspace": {
			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Times(0)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find an application attached to this workspace, please run `app init` first",
		},
		"error while fetching application": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Return(nil, errors.New("some error"))
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications in workspace: some error",
		},
		"error when application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Return([]string{"backend"}, nil)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "application 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().ServiceNames().Times(0)
				mockProjectService.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &config.ErrNoSuchEnvironment{
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
			mockProjectService = mocks.NewMockstore(ctrl)

			tc.setupMocks()

			opts := &packageAppOpts{
				packageAppVars: packageAppVars{
					Name:       tc.inAppName,
					EnvName:    tc.inEnvName,
					GlobalOpts: &GlobalOpts{appName: tc.inProjectName},
				},
				ws:    mockWorkspace,
				store: mockProjectService,
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

func TestPackageAppOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inTag     string

		expectWS     func(m *mocks.MockwsSvcReader)
		expectStore  func(m *mocks.Mockstore)
		expectPrompt func(m *mocks.Mockprompter)
		expectRunner func(m *mocks.Mockrunner)

		wantedAppName string
		wantedEnvName string
		wantedTag     string
		wantedErrorS  string
	}{
		"wrap list apps error": {
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return(nil, errors.New("some error"))
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedErrorS: "list applications in workspace: some error",
		},
		"empty workspace error": {
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{}, nil)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedErrorS: "there are no applications in the workspace, run `ecs-preview init` first",
		},
		"wrap list envs error": {
			inAppName: "frontend",
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Times(0)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, errors.New("some ssm error"))
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedErrorS:  "list environments for project : some ssm error",
		},
		"empty environments error": {
			inAppName: "frontend",
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Times(0)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedErrorS:  "there are no environments in project ",
		},
		"prompt for all options": {
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*config.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageAppNamePrompt, gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
				m.EXPECT().SelectOne(appPackageEnvNamePrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil)
				m.EXPECT().Get(inputImageTagPrompt, "", nil).Return("v1.0.0", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the app name": {
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageAppNamePrompt, gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the env name": {
			inAppName: "frontend",
			inTag:     "v1.0.0",

			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Times(0)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*config.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageEnvNamePrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt": {
			inAppName: "frontend",
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Times(0)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt if only one app or one env": {
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Return([]string{"frontend"}, nil)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*config.Environment{
					{
						Name: "test",
					},
				}, nil)
			},
			expectRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Get(inputImageTagPrompt, "", nil).Return("v1.0.0", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt for image if there is a runner error": {
			inAppName: "frontend",
			inEnvName: "test",
			expectWS: func(m *mocks.MockwsSvcReader) {
				m.EXPECT().ServiceNames().Times(0)
			},
			expectStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(inputImageTagPrompt, "", nil).Return("v1.0.0", nil)
			},
			expectRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockwsSvcReader(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)

			tc.expectWS(mockWorkspace)
			tc.expectStore(mockStore)
			tc.expectPrompt(mockPrompt)
			tc.expectRunner(mockRunner)

			opts := &packageAppOpts{
				packageAppVars: packageAppVars{
					Name:    tc.inAppName,
					EnvName: tc.inEnvName,
					Tag:     tc.inTag,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},
				ws:     mockWorkspace,
				store:  mockStore,
				runner: mockRunner,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedAppName, opts.Name)
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

func TestPackageAppOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inVars packageAppVars

		mockDependencies func(*gomock.Controller, *packageAppOpts)

		wantedStack  string
		wantedParams string
		wantedAddons string
		wantedErr    error
	}{
		"writes application template without addons": {
			inVars: packageAppVars{
				GlobalOpts: &GlobalOpts{
					appName: "ecs-kudos",
				},
				Name:    "api",
				EnvName: "test",
				Tag:     "1234",
			},
			mockDependencies: func(ctrl *gomock.Controller, opts *packageAppOpts) {
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
					Return("", &addons.ErrDirNotExist{})

				opts.store = mockStore
				opts.ws = mockWs
				opts.describer = mockCfn
				opts.initAddonsSvc = func(opts *packageAppOpts) error {
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
			opts := &packageAppOpts{
				packageAppVars: tc.inVars,

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
