// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"os"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestConfigurationSelector_Schedule(t *testing.T) {
	scheduleTypePrompt := "HAY WHAT SCHEDULE"
	scheduleTypeHelp := "NO"

	testCases := map[string]struct {
		mockPrompt     func(*mocks.MockPrompter)
		wantedSchedule string
		wantedErr      error
	}{
		"error asking schedule type": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get schedule type: some error"),
		},
		"ask for rate": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("1h30m", nil),
				)
			},
			wantedSchedule: "@every 1h30m",
		},
		"error getting rate": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("", fmt.Errorf("some error")),
				)
			},
			wantedErr: errors.New("get schedule rate: some error"),
		},
		"ask for cron": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Daily", nil),
				)
			},
			wantedSchedule: "@daily",
		},
		"error getting cron": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get preset schedule: some error"),
		},
		"ask for custom schedule": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("0 * * * *", nil),
					m.EXPECT().Confirm(humanReadableCronConfirmPrompt, humanReadableCronConfirmHelp).Return(true, nil),
				)
			},
			wantedSchedule: "0 * * * *",
		},
		"error getting custom schedule": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get custom schedule: some error"),
		},
		"error confirming custom schedule": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("0 * * * *", nil),
					m.EXPECT().Confirm(humanReadableCronConfirmPrompt, humanReadableCronConfirmHelp).Return(false, errors.New("some error")),
				)
			},
			wantedErr: errors.New("confirm cron schedule: some error"),
		},
		"custom schedule using valid definition string results in no confirm": {
			mockPrompt: func(m *mocks.MockPrompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("@hourly", nil),
				)
			},
			wantedSchedule: "@hourly",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			p := mocks.NewMockPrompter(ctrl)
			tc.mockPrompt(p)
			sel := staticSelector{
				prompt: p,
			}

			var mockValidator prompt.ValidatorFunc = func(interface{}) error { return nil }

			// WHEN
			schedule, err := sel.Schedule(scheduleTypePrompt, scheduleTypeHelp, mockValidator, mockValidator)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedSchedule, schedule)
			}
		})
	}
}

func TestLocalFileSelector_Dockerfile(t *testing.T) {
	testCases := map[string]struct {
		mockPrompt     func(*mocks.MockPrompter)
		mockFileSystem func(fs afero.Fs)

		wantedErr        error
		wantedDockerfile string
	}{
		"choose an existing Dockerfile": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll("frontend", 0755)
				_ = mockFS.MkdirAll("backend", 0755)

				_ = afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "backend/my.dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"./Dockerfile",
						"backend/my.dockerfile",
						"frontend/Dockerfile",
						dockerfilePromptUseCustom,
						DockerfilePromptUseImage,
					}),
					gomock.Any(),
				).Return("frontend/Dockerfile", nil)
			},
			wantedDockerfile: "frontend/Dockerfile",
		},
		"prompts user for custom path": {
			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						dockerfilePromptUseCustom,
						DockerfilePromptUseImage,
					}),
					gomock.Any(),
				).Return("Enter custom path for your Dockerfile", nil)
				m.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("crazy/path/Dockerfile", nil)
			},
			wantedDockerfile: "crazy/path/Dockerfile",
		},
		"returns an error if fail to select Dockerfile": {
			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().SelectOne(
					gomock.Any(),
					gomock.Any(),
					gomock.Eq([]string{
						dockerfilePromptUseCustom,
						DockerfilePromptUseImage,
					}),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"returns an error if fail to get custom Dockerfile path": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().SelectOne(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"./Dockerfile",
						dockerfilePromptUseCustom,
						DockerfilePromptUseImage,
					}),
					gomock.Any(),
				).Return("Enter custom path for your Dockerfile", nil)
				m.EXPECT().Get(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get custom Dockerfile path: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			p := mocks.NewMockPrompter(ctrl)
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			tc.mockPrompt(p)

			sel := dockerfileSelector{
				prompt: p,
				fs:     fs,
			}

			mockPromptText := "prompt"
			mockHelpText := "help"

			// WHEN
			dockerfile, err := sel.Dockerfile(
				mockPromptText,
				mockPromptText,
				mockHelpText,
				mockHelpText,
				func(v interface{}) error { return nil },
			)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedDockerfile, dockerfile)
			}
		})
	}
}

func TestLocalFileSelector_StaticSources(t *testing.T) {
	wd, _ := os.Getwd()
	mockFS := func(mockFS afero.Fs) {
		_ = mockFS.MkdirAll(wd+"this/path/to/projectRoot/copilot", 0755)
		_ = mockFS.MkdirAll(wd+"this/path/to/projectRoot/frontend", 0755)
		_ = mockFS.MkdirAll(wd+"this/path/to/projectRoot/backend", 0755)
		_ = mockFS.MkdirAll(wd+"this/path/to/projectRoot/friend", 0755)
		_ = mockFS.MkdirAll(wd+"this/path/to/projectRoot/trend", 0755)

		_ = afero.WriteFile(mockFS, wd+"this/path/to/projectRoot/myFile", []byte("cool stuff"), 0644)
		_ = afero.WriteFile(mockFS, wd+"this/path/to/projectRoot/frontend/feFile", []byte("css and stuff"), 0644)
		_ = afero.WriteFile(mockFS, wd+"this/path/to/projectRoot/backend/beFile", []byte("content stuff"), 0644)
	}
	testCases := map[string]struct {
		mockPrompt     func(*mocks.MockPrompter)
		mockFileSystem func(fs afero.Fs)

		wantedErr        error
		wantedDirOrFiles []string
	}{
		"successfully choose existing files, dirs, and multiple write-in paths": {
			mockFileSystem: mockFS,
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().MultiSelect(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"backend",
						"backend/beFile",
						"friend",
						"frontend",
						"frontend/feFile",
						"myFile",
						"trend",
						staticSourceUseCustomPrompt,
					}),
					gomock.Any(), gomock.Any(),
				).Return([]string{"myFile", "frontend", "backend/beFile", staticSourceUseCustomPrompt}, nil)
				m.EXPECT().Get(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("friend", nil)
				m.EXPECT().Confirm(
					gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				m.EXPECT().Get(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("trend", nil)
				m.EXPECT().Confirm(
					gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantedDirOrFiles: []string{"myFile", "frontend", "backend/beFile", "friend", "trend"},
		},
		"error with multiselect": {
			mockFileSystem: mockFS,
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().MultiSelect(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"backend",
						"backend/beFile",
						"friend",
						"frontend",
						"frontend/feFile",
						"myFile",
						"trend",
						staticSourceUseCustomPrompt,
					}),
					gomock.Any(), gomock.Any(),
				).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("select directories and/or files: some error"),
		},
		"error entering custom path": {
			mockFileSystem: mockFS,
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().MultiSelect(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"backend",
						"backend/beFile",
						"friend",
						"frontend",
						"frontend/feFile",
						"myFile",
						"trend",
						staticSourceUseCustomPrompt,
					}),
					gomock.Any(), gomock.Any(),
				).Return([]string{staticSourceUseCustomPrompt}, nil)
				m.EXPECT().Get(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get custom directory or file path: some error"),
		},
		"error confirming whether not to prompt for another custom path": {
			mockFileSystem: mockFS,
			mockPrompt: func(m *mocks.MockPrompter) {
				m.EXPECT().MultiSelect(
					gomock.Any(), gomock.Any(),
					gomock.Eq([]string{
						"backend",
						"backend/beFile",
						"friend",
						"frontend",
						"frontend/feFile",
						"myFile",
						"trend",
						staticSourceUseCustomPrompt,
					}),
					gomock.Any(), gomock.Any(),
				).Return([]string{staticSourceUseCustomPrompt}, nil)
				m.EXPECT().Get(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("friend", nil)
				m.EXPECT().Confirm(
					gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("confirm another custom path: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cwd, _ := os.Getwd()
			p := mocks.NewMockPrompter(ctrl)
			w := &workspace.Workspace{
				CopilotDirAbs: cwd + "this/path/to/projectRoot/copilot",
			}
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			tc.mockPrompt(p)

			sel := localFileSelector{
				prompt:        p,
				ws:            w,
				fs:            fs,
				workingDirAbs: "this/path/to/projectRoot",
			}

			mockPromptText := "prompt"
			mockHelpText := "help"

			// WHEN
			sourceFiles, err := sel.StaticSources(
				mockPromptText,
				mockPromptText,
				mockHelpText,
				mockHelpText,
				nil,
			)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedDirOrFiles, sourceFiles)
			}
		})
	}
}

func TestLocalFileSelector_listDockerfiles(t *testing.T) {
	testCases := map[string]struct {
		workingDirAbs  string
		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
		dockerfiles    []string
	}{
		"find Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll("frontend", 0755)
				_ = mockFS.MkdirAll("backend", 0755)

				_ = afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			dockerfiles: []string{"./Dockerfile", "backend/Dockerfile", "frontend/Dockerfile"},
		},
		"exclude dockerignore files": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll("frontend", 0755)
				_ = mockFS.MkdirAll("backend", 0755)

				_ = afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/Dockerfile.dockerignore", []byte("*/temp*"), 0644)
				_ = afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "backend/Dockerfile.dockerignore", []byte("*/temp*"), 0644)
			},
			wantedErr:   nil,
			dockerfiles: []string{"./Dockerfile", "backend/Dockerfile", "frontend/Dockerfile"},
		},
		"exclude Dockerfiles in parent directories of the working dir": {
			workingDirAbs: "/app",
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll("/app", 0755)
				_ = afero.WriteFile(mockFS, "/app/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "backend/my.dockerfile", []byte("FROM nginx"), 0644)
			},
			dockerfiles: []string{"./Dockerfile"},
		},
		"nonstandard Dockerfile names": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll("frontend", 0755)
				_ = mockFS.MkdirAll("dockerfiles", 0755)
				_ = afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "frontend/dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "Job.dockerfile", []byte("FROM nginx"), 0644)
				_ = afero.WriteFile(mockFS, "Job.dockerfile.dockerignore", []byte("*/temp*"), 0644)
			},
			dockerfiles: []string{"./Dockerfile", "./Job.dockerfile", "frontend/dockerfile"},
		},
		"no Dockerfiles": {
			mockFileSystem: func(mockFS afero.Fs) {},
			dockerfiles:    []string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			s := &dockerfileSelector{
				workingDirAbs: tc.workingDirAbs,
				fs: &afero.Afero{
					Fs: fs,
				},
			}

			got, err := s.listDockerfiles()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.dockerfiles, got)
			}
		})
	}
}

func TestLocalFileSelector_listDirsAndFiles(t *testing.T) {
	wd, _ := os.Getwd()
	testCases := map[string]struct {
		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
		dirsAndFiles   []string
	}{
		"drill down two (and only two) levels": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll(wd+"/projectRoot/lobby/copilot", 0755)
				_ = mockFS.MkdirAll(wd+"/projectRoot/lobby/basement/subBasement/subSubBasement/subSubSubBasement", 0755)

				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/file", []byte("cool stuff"), 0644)
				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/basement/file", []byte("more cool stuff"), 0644)
				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/basement/subBasement/file", []byte("unreachable cool stuff"), 0644)
			},
			dirsAndFiles: []string{"lobby", "lobby/basement", "lobby/basement/file", "lobby/basement/subBasement", "lobby/basement/subBasement/file", "lobby/basement/subBasement/subSubBasement", "lobby/file"},
		},
		"exclude hidden files and copilot dir": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll(wd+"/projectRoot/lobby/basement/subBasement/subSubBasement", 0755)
				_ = mockFS.Mkdir(wd+"/projectRoot/lobby/copilot", 0755)

				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/.file", []byte("cool stuff"), 0644)
				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/basement/file", []byte("more cool stuff"), 0644)
				_ = afero.WriteFile(mockFS, wd+"/projectRoot/lobby/basement/subBasement/file", []byte("unreachable cool stuff"), 0644)
			},
			wantedErr:    nil,
			dirsAndFiles: []string{"lobby", "lobby/basement", "lobby/basement/file", "lobby/basement/subBasement", "lobby/basement/subBasement/file", "lobby/basement/subBasement/subSubBasement"},
		},
		"no dirs or files found": {
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.Mkdir(wd+"/projectRoot/copilot", 0755)
			},
			dirsAndFiles: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			cwd, _ := os.Getwd()
			w := &workspace.Workspace{
				CopilotDirAbs: cwd + "/projectRoot/copilot",
			}
			tc.mockFileSystem(fs)
			s := &localFileSelector{
				fs: &afero.Afero{
					Fs: fs,
				},
				ws:            w,
				workingDirAbs: "",
			}

			got, err := s.listDirsAndFiles()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.dirsAndFiles, got)
			}
		})
	}
}
