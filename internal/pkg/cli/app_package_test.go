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

		expectWS       func(m *mocks.MockWorkspace)
		expectEnvStore func(m *mocks.MockEnvironmentStore)
		expectPrompt   func(m *climocks.Mockprompter)

		wantedAppName string
		wantedEnvName string
		wantedErrorS  string
	}{
		"wrap list apps error": {
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return(nil, errors.New("some error"))
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications in workspace: some error",
		},
		"empty workspace error": {
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return([]string{}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "there are no applications in the workspace, run `archer init` first",
		},
		"wrap list envs error": {
			inAppName: "frontend",
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Return(nil, errors.New("some ssm error"))
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedAppName: "frontend",
			wantedErrorS:  "list environments for project : some ssm error",
		},
		"prompt for all options": {
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
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
				m.EXPECT().SelectOne(appPackageAppNamePrompt, gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
				m.EXPECT().SelectOne(appPackageEnvNamePrompt, gomock.Any(), []string{"test", "prod"}).Return("test", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"prompt only for the app name": {
			inEnvName: "test",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return([]string{"frontend", "backend"}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appPackageAppNamePrompt, gomock.Any(), []string{"frontend", "backend"}).Return("frontend", nil)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"prompt only for the env name": {
			inAppName: "frontend",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
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

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
		"don't prompt": {
			inAppName: "frontend",
			inEnvName: "test",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().ListEnvironments(gomock.Any()).Times(0)
			},
			expectPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},

			wantedAppName: "frontend",
			wantedEnvName: "test",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)

			tc.expectWS(mockWorkspace)
			tc.expectEnvStore(mockEnvStore)
			tc.expectPrompt(mockPrompt)

			opts := &PackageAppOpts{
				AppName:    tc.inAppName,
				EnvName:    tc.inEnvName,
				ws:         mockWorkspace,
				envStore:   mockEnvStore,
				prompt:     mockPrompt,
				GlobalOpts: &GlobalOpts{},
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedAppName, opts.AppName)
			require.Equal(t, tc.wantedEnvName, opts.EnvName)

			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestPackageAppOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inEnvName     string
		inAppName     string
		inTag         string

		expectWS       func(m *mocks.MockWorkspace)
		expectEnvStore func(m *mocks.MockEnvironmentStore)

		wantedErrorS string
	}{
		"invalid workspace": {
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "could not find a project attached to this workspace, please run `project init` first",
		},
		"invalid image tag": {
			inProjectName: "phonetool",
			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "image tag cannot be empty, please provide the `--tag` flag",
		},
		"error while fetching application": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inTag:         "manual-1234",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return(nil, errors.New("some error"))
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list applications in workspace: some error",
		},
		"error when application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inTag:         "manual-1234",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Return([]string{"backend"}, nil)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "application 'frontend' does not exist in the workspace",
		},
		"error while fetching environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inTag:         "manual-1234",

			expectWS: func(m *mocks.MockWorkspace) {
				m.EXPECT().AppNames().Times(0)
			},
			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &store.ErrNoSuchEnvironment{
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

			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			tc.expectWS(mockWorkspace)
			tc.expectEnvStore(mockEnvStore)

			opts := &PackageAppOpts{
				AppName: tc.inAppName,
				EnvName: tc.inEnvName,
				Tag:     tc.inTag,

				ws:       mockWorkspace,
				envStore: mockEnvStore,

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

		expectEnvStore  func(m *mocks.MockEnvironmentStore)
		expectWorkspace func(m *mocks.MockWorkspace)
		expectFS        func(t *testing.T, mockFS *afero.Afero)

		wantedErr error
	}{
		"invalid environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",

			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, &store.ErrNoSuchEnvironment{
					ProjectName:     "phonetool",
					EnvironmentName: "test",
				})
			},
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ReadManifestFile(gomock.Any()).Times(0)
			},

			wantedErr: &store.ErrNoSuchEnvironment{
				ProjectName:     "phonetool",
				EnvironmentName: "test",
			},
		},
		"invalid manifest file": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}, nil)
			},
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadManifestFile("frontend-app.yml").Return(nil, &workspace.ErrManifestNotFound{
					ManifestName: "frontend-app.yml",
				})
			},

			wantedErr: &workspace.ErrManifestNotFound{
				ManifestName: "frontend-app.yml",
			},
		},
		"invalid manifest type": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",

			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project: "phonetool",
					Name:    "test",
				}, nil)
			},
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadManifestFile("frontend-app.yml").Return([]byte("somecontent"), nil)
			},

			wantedErr: &manifest.ErrUnmarshalAppManifest{},
		},
		"print CFN template": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",

			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
			},
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadManifestFile("frontend-app.yml").Return([]byte(`name: frontend
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
		},
		"with output directory": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			inAppName:     "frontend",
			inTagName:     "latest",
			inOutputDir:   "./infrastructure",

			expectEnvStore: func(m *mocks.MockEnvironmentStore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					AccountID: "1111",
					Region:    "us-west-2",
				}, nil)
			},
			expectWorkspace: func(m *mocks.MockWorkspace) {
				m.EXPECT().ManifestFileName("frontend").Return("frontend-app.yml")
				m.EXPECT().ReadManifestFile("frontend-app.yml").Return([]byte(`name: frontend
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

			mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			tc.expectEnvStore(mockEnvStore)
			tc.expectWorkspace(mockWorkspace)

			templateBuf := &strings.Builder{}
			paramsBuf := &strings.Builder{}
			mockFS := &afero.Afero{Fs: afero.NewMemMapFs()}
			opts := PackageAppOpts{
				EnvName:   tc.inEnvName,
				AppName:   tc.inAppName,
				Tag:       tc.inTagName,
				OutputDir: tc.inOutputDir,

				envStore:     mockEnvStore,
				ws:           mockWorkspace,
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
