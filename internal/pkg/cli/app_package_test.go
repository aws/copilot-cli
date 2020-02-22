// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestPackageAppOpts_Validate(t *testing.T) {
	var (
		mockWorkspace      *climocks.MockwsAppReader
		mockProjectService *climocks.MockprojectService
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
				mockWorkspace.EXPECT().AppNames().Times(0)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find a project attached to this workspace, please run `project init` first",
		},
		"error while fetching application": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().AppNames().Return(nil, errors.New("some error"))
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications in workspace: some error",
		},
		"error when application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().AppNames().Return([]string{"backend"}, nil)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "application 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().AppNames().Times(0)
				mockProjectService.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &store.ErrNoSuchEnvironment{
					ProjectName:     "phonetool",
					EnvironmentName: "test",
				})
			},

			wantedErrorS: (&store.ErrNoSuchEnvironment{
				ProjectName:     "phonetool",
				EnvironmentName: "test",
			}).Error(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace = climocks.NewMockwsAppReader(ctrl)
			mockProjectService = climocks.NewMockprojectService(ctrl)

			tc.setupMocks()

			opts := &packageAppOpts{
				packageAppVars: packageAppVars{
					AppName:    tc.inAppName,
					EnvName:    tc.inEnvName,
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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

		expectWS     func(m *climocks.MockwsAppReader)
		expectStore  func(m *climocks.MockprojectService)
		expectPrompt func(m *climocks.Mockprompter)
		expectRunner func(m *climocks.Mockrunner)

		wantedAppName string
		wantedEnvName string
		wantedTag     string
		wantedErrorS  string
	}{
		"wrap list apps error": {
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return(nil, errors.New("some error"))
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedErrorS: "list applications in workspace: some error",
		},
		"empty workspace error": {
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{}, nil)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedErrorS: "there are no applications in the workspace, run `ecs-preview init` first",
		},
		"wrap list envs error": {
			inAppName: "frontend",
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Times(0)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, errors.New("some ssm error"))
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedErrorS:  "list environments for project : some ssm error",
		},
		"empty environments error": {
			inAppName: "frontend",
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Times(0)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, nil)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedErrorS:  "there are no environments in project ",
		},
		"prompt for all options": {
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectRunner: func(m *climocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectPrompt: func(m *climocks.Mockprompter) {
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

			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageAppNamePrompt, gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the env name": {
			inAppName: "frontend",
			inTag:     "v1.0.0",

			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Times(0)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod",
					},
				}, nil)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageEnvNamePrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt": {
			inAppName: "frontend",
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Times(0)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedAppName: "frontend",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt if only one app or one env": {
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend"}, nil)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return([]*archer.Environment{
					{
						Name: "test",
					},
				}, nil)
			},
			expectRunner: func(m *climocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectPrompt: func(m *climocks.Mockprompter) {
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
			expectWS: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Times(0)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(inputImageTagPrompt, "", nil).Return("v1.0.0", nil)
			},
			expectRunner: func(m *climocks.Mockrunner) {
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

			mockWorkspace := climocks.NewMockwsAppReader(ctrl)
			mockStore := climocks.NewMockprojectService(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)
			mockRunner := climocks.NewMockrunner(ctrl)

			tc.expectWS(mockWorkspace)
			tc.expectStore(mockStore)
			tc.expectPrompt(mockPrompt)
			tc.expectRunner(mockRunner)

			opts := &packageAppOpts{
				packageAppVars: packageAppVars{
					AppName: tc.inAppName,
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
			require.Equal(t, tc.wantedAppName, opts.AppName)
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
	mockErr := errors.New("some error")

	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inAppName     string
		inTagName     string
		inOutputDir   string

		expectStore     func(m *climocks.MockprojectService)
		expectWorkspace func(m *climocks.MockwsAppReader)
		expectDeployer  func(m *climocks.MockprojectResourcesGetter)
		expectFS        func(t *testing.T, mockFS *afero.Afero)
		expectAddonsSvc func(m *climocks.Mocktemplater)

		wantedErr error
	}{
		"invalid environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &store.ErrNoSuchEnvironment{
					ProjectName:     "phonetool",
					EnvironmentName: "test",
				})
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest(gomock.Any()).Times(0)
			},
			expectDeployer:  func(m *climocks.MockprojectResourcesGetter) {},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: &store.ErrNoSuchEnvironment{
				ProjectName:     "phonetool",
				EnvironmentName: "test",
			},
		},
		"invalid manifest file": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return(nil, mockErr)
			},
			expectDeployer:  func(m *climocks.MockprojectResourcesGetter) {},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: mockErr,
		},
		"invalid manifest type": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte("somecontent"), nil)
			},
			expectDeployer:  func(m *climocks.MockprojectResourcesGetter) {},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: &manifest.ErrUnmarshalAppManifest{},
		},
		"error while getting project from store": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(nil, &store.ErrNoSuchProject{ProjectName: "phonetool"})
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App`), nil)
			},
			expectDeployer:  func(m *climocks.MockprojectResourcesGetter) {},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: &store.ErrNoSuchProject{ProjectName: "phonetool"},
		},
		"error while getting regional resources from describer": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "1111",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, "us-west-2").Return(nil, &cloudformation.ErrStackSetOutOfDate{})
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: &cloudformation.ErrStackSetOutOfDate{},
		},
		"error if the repository does not exist": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "1111",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, "us-west-2").Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {},

			wantedErr: &errRepoNotFound{
				appName:       "frontend",
				envRegion:     "us-west-2",
				projAccountID: "1234",
			},
		},
		"error if fail to get addons template": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 256
memory: 512
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {
				m.EXPECT().Template().Return("", mockErr)
			},

			wantedErr: mockErr,
		},
		"print CFN template": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 256
memory: 512
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {
				m.EXPECT().Template().Return(`AWSTemplateFormatVersion: 2010-09-09
Description: Additional resources for application 'my-app'
Parameters:
	Project:
		Type: String
		Description: The project name.
	Env:
		Type: String
		Description: The environment name your application is being deployed to.
	App:
		Type: String
		Description: The name of the application being deployed.`, nil)
			},
		},
		"print CFN template with HTTPS": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
					Domain:    "ecs.aws",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 256
memory: 512
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {
				m.EXPECT().Template().Return(`AWSTemplateFormatVersion: 2010-09-09
Description: Additional resources for application 'my-app'
Parameters:
	Project:
		Type: String
		Description: The project name.
	Env:
		Type: String
		Description: The environment name your application is being deployed to.
	App:
		Type: String
		Description: The name of the application being deployed.`, nil)
			},
		},
		"with output directory": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",
			inOutputDir:   "./infrastructure",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 256
memory: 512
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {
				m.EXPECT().Template().Return(`AWSTemplateFormatVersion: 2010-09-09
Description: Additional resources for application 'my-app'
Parameters:
	Project:
		Type: String
		Description: The project name.
	Env:
		Type: String
		Description: The environment name your application is being deployed to.
	App:
		Type: String
		Description: The name of the application being deployed.`, nil)
			},
			expectFS: func(t *testing.T, mockFS *afero.Afero) {
				stackPath := filepath.Join("infrastructure", "frontend.stack.yml")
				stackFileExists, _ := mockFS.Exists(stackPath)
				require.True(t, stackFileExists, "expected file %s to exists", stackPath)

				paramsPath := filepath.Join("infrastructure", "frontend-test.params.json")
				paramsFileExists, _ := mockFS.Exists(paramsPath)
				require.True(t, paramsFileExists, "expected file %s to exists", paramsPath)

				addonsPath := filepath.Join("infrastructure", "frontend.addons.stack.yml")
				addonsFileExists, _ := mockFS.Exists(addonsPath)
				require.True(t, addonsFileExists, "expected file %s to exists", addonsPath)
			},
		},
		"with output directory and with no addons directory": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",
			inOutputDir:   "./infrastructure",

			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
				m.EXPECT().GetProject("phonetool").Return(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, nil)
			},
			expectWorkspace: func(m *climocks.MockwsAppReader) {
				m.EXPECT().ReadAppManifest("frontend").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 256
memory: 512
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectAddonsSvc: func(m *climocks.Mocktemplater) {
				m.EXPECT().Template().Return("", &workspace.ErrAddonsDirNotExist{
					AppName: "frontend",
				})
			},
			expectFS: func(t *testing.T, mockFS *afero.Afero) {
				stackPath := filepath.Join("infrastructure", "frontend.stack.yml")
				stackFileExists, _ := mockFS.Exists(stackPath)
				require.True(t, stackFileExists, "expected file %s to exists", stackPath)

				paramsPath := filepath.Join("infrastructure", "frontend-test.params.json")
				paramsFileExists, _ := mockFS.Exists(paramsPath)
				require.True(t, paramsFileExists, "expected file %s to exists", paramsPath)

				addonsPath := filepath.Join("infrastructure", "frontend.addons.stack.yml")
				addonsFileExists, _ := mockFS.Exists(addonsPath)
				require.False(t, addonsFileExists, "expected file %s to exists", addonsPath)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := climocks.NewMockprojectService(ctrl)
			mockWorkspace := climocks.NewMockwsAppReader(ctrl)
			mockDeployer := climocks.NewMockprojectResourcesGetter(ctrl)
			mockAddonSvc := climocks.NewMocktemplater(ctrl)
			tc.expectStore(mockStore)
			tc.expectWorkspace(mockWorkspace)
			tc.expectDeployer(mockDeployer)
			tc.expectAddonsSvc(mockAddonSvc)

			templateBuf := &strings.Builder{}
			paramsBuf := &strings.Builder{}
			addonsBuf := &strings.Builder{}
			mockFS := &afero.Afero{Fs: afero.NewMemMapFs()}
			opts := packageAppOpts{
				packageAppVars: packageAppVars{
					EnvName:    tc.inEnvName,
					AppName:    tc.inAppName,
					Tag:        tc.inTagName,
					OutputDir:  tc.inOutputDir,
					GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
				},

				addonsSvc:     mockAddonSvc,
				initAddonsSvc: func(*packageAppOpts) error { return nil },
				store:         mockStore,
				ws:            mockWorkspace,
				describer:     mockDeployer,
				stackWriter:   templateBuf,
				paramsWriter:  paramsBuf,
				addonsWriter:  addonsBuf,
				fs:            mockFS,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr != nil {
				require.True(t, errors.Is(err, tc.wantedErr), "expected %v but got %v", tc.wantedErr, err)
				return
			}
			require.Nil(t, err, "expected no errors but got %v", err)
			if tc.inOutputDir != "" {
				tc.expectFS(t, mockFS)
			} else {
				require.Greater(t, len(templateBuf.String()), 0, "expected a template to be rendered %s", templateBuf.String())
				require.Greater(t, len(paramsBuf.String()), 0, "expected parameters to be rendered %s", paramsBuf.String())
			}
		})
	}
}
