// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestPackageJobOpts_Validate(t *testing.T) {
	var (
		mockWorkspace *mocks.MockwsJobDirReader
		mockStore     *mocks.Mockstore
	)

	testCases := map[string]struct {
		inAppName string
		inEnvName string
		inJobName string

		setupMocks func()

		wantedErrorS string
	}{
		"invalid workspace": {
			setupMocks: func() {
				mockWorkspace.EXPECT().JobNames().Times(0)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErrorS: "could not find an application attached to this workspace, please run `app init` first",
		},
		"error while fetching job": {
			inAppName: "phonetool",
			inJobName: "resizer",
			setupMocks: func() {
				mockWorkspace.EXPECT().JobNames().Return(nil, errors.New("some error"))
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "list jobs in the workspace: some error",
		},
		"error when job not in workspace": {
			inAppName: "phonetool",
			inJobName: "resizer",
			setupMocks: func() {
				mockWorkspace.EXPECT().JobNames().Return([]string{"other-job"}, nil)
				mockStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
			},

			wantedErrorS: "job 'resizer' does not exist in the workspace",
		},
		"error while fetching environment": {
			inAppName: "phonetool",
			inEnvName: "test",

			setupMocks: func() {
				mockWorkspace.EXPECT().JobNames().Times(0)
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

			mockWorkspace = mocks.NewMockwsJobDirReader(ctrl)
			mockStore = mocks.NewMockstore(ctrl)

			tc.setupMocks()

			opts := &packageJobOpts{
				packageJobVars: packageJobVars{
					name:    tc.inJobName,
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

func TestPackageJobOpts_Ask(t *testing.T) {
	const testAppName = "phonetool"
	testCases := map[string]struct {
		inJobName string
		inEnvName string
		inTag     string

		expectSelector func(m *mocks.MockwsSelector)
		expectPrompt   func(m *mocks.Mockprompter)
		expectRunner   func(m *mocks.Mockrunner)

		wantedJobName string
		wantedEnvName string
		wantedTag     string
		wantedErrorS  string
	}{
		"prompt for all options": {
			expectRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("not a git repo"))
			},
			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(jobPackageJobNamePrompt, "").Return("resizer", nil)
				m.EXPECT().Environment(jobPackageEnvNamePrompt, "", testAppName).Return("test", nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {},

			wantedJobName: "resizer",
			wantedEnvName: "test",
			wantedTag:     "", // No tag if there is no git repository.
		},
		"prompt only for the job name": {
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(jobPackageJobNamePrompt, "").Return("resizer", nil)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedJobName: "resizer",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"prompt only for the env name": {
			inJobName: "resizer",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(jobPackageEnvNamePrompt, "", testAppName).Return("test", nil)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedJobName: "resizer",
			wantedEnvName: "test",
			wantedTag:     "v1.0.0",
		},
		"don't prompt": {
			inJobName: "resizer",
			inEnvName: "test",
			inTag:     "v1.0.0",

			expectSelector: func(m *mocks.MockwsSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any()).Times(0)
				m.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			expectRunner: func(m *mocks.Mockrunner) {},

			wantedJobName: "resizer",
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

			opts := &packageJobOpts{
				packageJobVars: packageJobVars{
					name:    tc.inJobName,
					envName: tc.inEnvName,
					tag:     tc.inTag,
					appName: testAppName,
				},
				sel:    mockSelector,
				prompt: mockPrompt,
				runner: mockRunner,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			require.Equal(t, tc.wantedJobName, opts.name)
			require.Equal(t, tc.wantedEnvName, opts.envName)
			require.Equal(t, tc.wantedTag, opts.tag)

			if tc.wantedErrorS != "" {
				require.EqualError(t, err, tc.wantedErrorS)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPackageJobOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inVars packageJobVars

		mockDependencies func(*gomock.Controller, *packageJobOpts)

		wantedErr error
	}{
		"writes job template without addons": {
			inVars: packageJobVars{
				appName: "ecs-kudos",
				name:    "resizer",
				envName: "test",
				tag:     "1234",
			},
			mockDependencies: func(ctrl *gomock.Controller, opts *packageJobOpts) {
				opts.newPackageCmd = func(opts *packageJobOpts) {
					mockCmd := mocks.NewMockactionCommand(ctrl)
					mockCmd.EXPECT().Execute().Return(nil)
					opts.packageCmd = mockCmd
				}
			},
			wantedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &packageJobOpts{
				packageJobVars: tc.inVars,
				packageCmd:     mocks.NewMockactionCommand(ctrl),
			}
			tc.mockDependencies(ctrl, opts)

			// WHEN
			err := opts.Execute()

			// THEN
			require.Equal(t, tc.wantedErr, err)
		})
	}
}
