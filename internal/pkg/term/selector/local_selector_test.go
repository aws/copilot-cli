// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"
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
		mockPrompt     func(*mocks.Mockprompter)
		wantedSchedule string
		wantedErr      error
	}{
		"error asking schedule type": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get schedule type: some error"),
		},
		"ask for rate": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("1h30m", nil),
				)
			},
			wantedSchedule: "@every 1h30m",
		},
		"error getting rate": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(rate, nil),
					m.EXPECT().Get(ratePrompt, rateHelp, gomock.Any(), gomock.Any()).Return("", fmt.Errorf("some error")),
				)
			},
			wantedErr: errors.New("get schedule rate: some error"),
		},
		"ask for cron": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Daily", nil),
				)
			},
			wantedSchedule: "@daily",
		},
		"error getting cron": {
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get preset schedule: some error"),
		},
		"ask for custom schedule": {
			mockPrompt: func(m *mocks.Mockprompter) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
				gomock.InOrder(
					m.EXPECT().SelectOne(scheduleTypePrompt, scheduleTypeHelp, scheduleTypes, gomock.Any()).Return(fixedSchedule, nil),
					m.EXPECT().SelectOption(schedulePrompt, scheduleHelp, presetSchedules, gomock.Any()).Return("Custom", nil),
					m.EXPECT().Get(customSchedulePrompt, customScheduleHelp, gomock.Any(), gomock.Any()).Return("", errors.New("some error")),
				)
			},
			wantedErr: errors.New("get custom schedule: some error"),
		},
		"error confirming custom schedule": {
			mockPrompt: func(m *mocks.Mockprompter) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
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
			p := mocks.NewMockprompter(ctrl)
			tc.mockPrompt(p)
			sel := configurationSelector{
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
		mockPrompt     func(*mocks.Mockprompter)
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
			mockPrompt: func(m *mocks.Mockprompter) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
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

			p := mocks.NewMockprompter(ctrl)
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			tc.mockFileSystem(fs)
			tc.mockPrompt(p)

			sel := localFileSelector{
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
			s := &localFileSelector{
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
