// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type jobLogsMock struct {
	configStore *mocks.Mockstore
	sel         *mocks.MockdeploySelector
}

func TestJobLogs_Validate(t *testing.T) {
	const (
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputApp       string
		inputSvc       string
		inputLimit     int
		inputFollow    bool
		inputEnvName   string
		inputStartTime string
		inputEndTime   string
		inputSince     time.Duration

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			mockstore: func(m *mocks.Mockstore) {},

			wantedError: nil,
		},
		"invalid project name": {
			inputApp: "my-app",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"returns error if since and startTime flags are set together": {
			inputSince:     mockSince,
			inputStartTime: mockStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --since or --start-time may be used"),
		},
		"returns error if follow and endTime flags are set together": {
			inputFollow:  true,
			inputEndTime: mockEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --follow or --end-time may be used"),
		},
		"returns error if invalid start time flag value": {
			inputStartTime: mockBadStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badStartTime for \"--start-time\" flag: reading time value badStartTime: parsing time \"badStartTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badStartTime\" as \"2006\""),
		},
		"returns error if invalid end time flag value": {
			inputEndTime: mockBadEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badEndTime for \"--end-time\" flag: reading time value badEndTime: parsing time \"badEndTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badEndTime\" as \"2006\""),
		},
		"returns error if invalid since flag value": {
			inputSince: -mockSince,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--since must be greater than 0"),
		},
		"returns error if limit value is below limit": {
			inputLimit: -1,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit -1 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if limit value is above limit": {
			inputLimit: 10001,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit 10001 is out-of-bounds, value must be between 1 and 10000"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			tc.mockstore(mockstore)

			jobLogs := &jobLogsOpts{
				jobLogsVars: jobLogsVars{
					wkldLogsVars: wkldLogsVars{
						follow:         tc.inputFollow,
						limit:          tc.inputLimit,
						envName:        tc.inputEnvName,
						humanStartTime: tc.inputStartTime,
						humanEndTime:   tc.inputEndTime,
						since:          tc.inputSince,
						name:           tc.inputSvc,
						appName:        tc.inputApp,
					},
				},
				wkldLogOpts: wkldLogOpts{
					configStore: mockstore,
				},
			}

			// WHEN
			err := jobLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
