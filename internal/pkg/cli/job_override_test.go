// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestOverrideJob_Ask(t *testing.T) {
	t.Run("ask or validate job name", func(t *testing.T) {
		testCases := map[string]struct {
			name      string
			initMocks func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts)

			wanted error
		}{
			"validation passes if job exists in local workspace": {
				name: "reporter",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListJobs().Return([]string{"backend", "reporter", "worker"}, nil)
					cmd.ws = mockWS
				},
			},
			"return a wrapped error if local jobs cannot be retrieved from workspace": {
				name: "reporter",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListJobs().Return(nil, errors.New("some error"))
					cmd.ws = mockWS
				},
				wanted: errors.New("list jobs in the workspace: some error"),
			},
			"return an error if job does not exist in the workspace": {
				name: "reporter",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockWS := mocks.NewMockwsWlDirReader(ctrl)
					mockWS.EXPECT().ListJobs().Return([]string{"payments"}, nil)
					cmd.ws = mockWS
				},
				wanted: errors.New(`job "reporter" does not exist in the workspace`),
			},
			"should ask for the local job name if flag is not provided": {
				name: "",
				initMocks: func(ctrl *gomock.Controller, cmd *overrideWorkloadOpts) {
					mockPrompt := mocks.NewMockwsSelector(ctrl)
					mockPrompt.EXPECT().Job(gomock.Any(), gomock.Any())
					cmd.wsPrompt = mockPrompt
				},
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				// GIVEN
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()
				mockCfnPrompt := mocks.NewMockcfnSelector(ctrl)
				mockCfnPrompt.EXPECT().Resources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				vars := overrideVars{name: tc.name, appName: "demo", iacTool: "cdk"}
				cmd := &overrideWorkloadOpts{
					overrideOpts: &overrideOpts{
						overrideVars: vars,
						cfgStore:     mocks.NewMockstore(ctrl),
						cfnPrompt:    mockCfnPrompt,
						packageCmd: func(_ stringWriteCloser) (executor, error) {
							mockCmd := mocks.NewMockexecutor(ctrl)
							mockCmd.EXPECT().Execute().AnyTimes()
							return mockCmd, nil
						},
						spinner: &spinnerTestDouble{},
					},
				}
				cmd.validateOrAskName = cmd.validateOrAskJobName
				tc.initMocks(ctrl, cmd)

				// WHEN
				err := cmd.Ask()

				// THEN
				if tc.wanted != nil {
					require.EqualError(t, err, tc.wanted.Error())
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}
