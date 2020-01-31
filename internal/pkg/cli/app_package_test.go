// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestPackageAppOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inTag     string

		expectWS     func(m *mocks.MockWorkspace)
		expectStore  func(m *climocks.MockprojectService)
		expectPrompt func(m *climocks.Mockprompter)
		expectRunner func(m *climocks.Mockrunner)

		wantedAppName string
		wantedEnvName string
		wantedTag     string
		wantedErrorS  string
	}{
		"wrap list apps error": {
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Return(nil, errors.New("some error"))
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
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Return([]archer.Manifest{}, nil)
			},
			expectStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *climocks.Mockrunner) {},

			wantedErrorS: "there are no applications in the workspace, run `dw_run.sh init` first",
		},
		"wrap list envs error": {
			inAppName: "frontend",
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Times(0)
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
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Times(0)
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
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Return([]archer.Manifest{
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "frontend",
						},
					},
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "backend",
						},
					}}, nil)
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

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Return([]archer.Manifest{
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "frontend",
						},
					},
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "backend",
						},
					}}, nil)
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

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Times(0)
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

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Times(0)
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
		"prompt for image if there is a runner error": {
			inAppName: "frontend",
			inEnvName: "test",
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().Apps().Times(0)
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

			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockStore := climocks.NewMockprojectService(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)
			mockRunner := climocks.NewMockrunner(ctrl)

			tc.expectWS(mockWorkspace)
			tc.expectStore(mockStore)
			tc.expectPrompt(mockPrompt)
			tc.expectRunner(mockRunner)

			opts := &PackageAppOpts{
				AppName: tc.inAppName,
				EnvName: tc.inEnvName,
				Tag:     tc.inTag,
				ws:      mockWorkspace,
				store:   mockStore,
				runner:  mockRunner,
				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
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

func TestPackageAppOpts_Validate(t *testing.T) {
	var (
		mockWorkspace      *mocks.MockWorkspace
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
				mockWorkspace.EXPECT().Apps().Times(0)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find a project attached to this workspace, please run `project init` first",
		},
		"error while fetching application": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().Apps().Return(nil, errors.New("some error"))
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications in workspace: some error",
		},
		"error when application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			setupMocks: func() {
				mockWorkspace.EXPECT().Apps().Return([]archer.Manifest{
					&manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: "backend",
						},
					},
				}, nil)
				mockProjectService.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "application 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().Apps().Times(0)
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

			mockWorkspace = mocks.NewMockWorkspace(ctrl)
			mockProjectService = climocks.NewMockprojectService(ctrl)

			tc.setupMocks()

			opts := &PackageAppOpts{
				AppName:    tc.inAppName,
				EnvName:    tc.inEnvName,
				ws:         mockWorkspace,
				store:      mockProjectService,
				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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

func TestPackageAppOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inAppName     string
		inTagName     string
		inOutputDir   string

		expectStore     func(m *climocks.MockprojectService)
		expectWorkspace func(m *mocks.MockWorkspace)
		expectDeployer  func(m *climocks.MockprojectResourcesGetter)
		expectFS        func(t *testing.T, mockFS *afero.Afero)

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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ReadFile(gomock.Any()).Times(0)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {},

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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return(nil, &workspace.ErrManifestNotFound{
					ManifestName: "frontend-app.yml",
				})
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {},

			wantedErr: &workspace.ErrManifestNotFound{
				ManifestName: "frontend-app.yml",
			},
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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte("somecontent"), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {},

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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
type: Load Balanced Web App`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {},

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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
type: Load Balanced Web App`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(&archer.Project{
					Name:      "phonetool",
					AccountID: "1234",
				}, "us-west-2").Return(nil, &cloudformation.ErrStackSetOutOfDate{})
			},
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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
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
			wantedErr: &errRepoNotFound{
				appName:       "frontend",
				envRegion:     "us-west-2",
				projAccountID: "1234",
			},
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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 512
memory: 1024
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 512
memory: 1024
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
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
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadFile("frontend-app.yml").Return([]byte(`name: frontend
type: Load Balanced Web App
image:
  build: frontend/Dockerfile
  port: 80
http:
  path: '*'
cpu: 512
memory: 1024
count: 1`), nil)
			},
			expectDeployer: func(m *climocks.MockprojectResourcesGetter) {
				m.EXPECT().GetProjectResourcesByRegion(gomock.Any(), gomock.Any()).Return(&archer.ProjectRegionalResources{
					RepositoryURLs: map[string]string{
						"frontend": "some url",
					},
				}, nil)
			},
			expectFS: func(t *testing.T, mockFS *afero.Afero) {
				stackPath := filepath.Join("infrastructure", "frontend.stack.yml")
				stackFileExists, _ := mockFS.Exists(stackPath)
				require.True(t, stackFileExists, "expected file %s to exists", stackPath)

				paramsPath := filepath.Join("infrastructure", "frontend-test.params.json")
				paramsFileExists, _ := mockFS.Exists(paramsPath)
				require.True(t, paramsFileExists, "expected file %s to exists", paramsFileExists)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := climocks.NewMockprojectService(ctrl)
			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockDeployer := climocks.NewMockprojectResourcesGetter(ctrl)
			tc.expectStore(mockStore)
			tc.expectWorkspace(mockWorkspace)
			tc.expectDeployer(mockDeployer)

			templateBuf := &strings.Builder{}
			paramsBuf := &strings.Builder{}
			mockFS := &afero.Afero{Fs: afero.NewMemMapFs()}
			opts := PackageAppOpts{
				EnvName:   tc.inEnvName,
				AppName:   tc.inAppName,
				Tag:       tc.inTagName,
				OutputDir: tc.inOutputDir,

				store:        mockStore,
				ws:           mockWorkspace,
				describer:    mockDeployer,
				stackWriter:  templateBuf,
				paramsWriter: paramsBuf,
				fs:           mockFS,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
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
