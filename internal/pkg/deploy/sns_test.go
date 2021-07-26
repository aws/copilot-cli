// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	mockGoodARN = "arn:aws:sns:us-west-2:12345678012:app-env-svc-topic"
	mockBadARN  = "arn:aws:sns:us-west-2:12345678012:topic"
	mockApp     = "app"
	mockEnv     = "env"
	mockSvc     = "svc"
)

func TestTopic_Name(t *testing.T) {

	testCases := map[string]struct {
		inputTopic Topic

		wanted      string
		wantedError error
	}{
		"good arn": {
			inputTopic: Topic{
				ARN:  mockGoodARN,
				App:  mockApp,
				Env:  mockEnv,
				Wkld: mockSvc,
			},
			wanted: "topic",
		},
		"bad arn format": {
			inputTopic: Topic{
				ARN: "badARN",
			},
			wantedError: errInvalidARN,
		},
		"bad arn: for non-copilot topic": {
			inputTopic: Topic{
				ARN:  mockBadARN,
				App:  mockApp,
				Env:  mockEnv,
				Wkld: mockSvc,
			},
			wantedError: errInvalidTopicARN,
		},
		"bad arn: arn for non-sns service": {
			inputTopic: Topic{
				ARN: "arn:aws:s3:::bucketname",
			},
			wantedError: errInvalidARNService,
		},
		"bad arn: arn for copilot topic subscription": {
			inputTopic: Topic{
				ARN:  mockGoodARN + ":12345-abcde-12345-abcde",
				App:  mockApp,
				Env:  mockEnv,
				Wkld: mockSvc,
			},
			wantedError: errInvalidARNService,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			name, err := tc.inputTopic.Name()
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, name)
			}
		})
	}
}

func TestTopic_ID(t *testing.T) {
	testCases := map[string]struct {
		inputTopic Topic

		wanted      string
		wantedError error
	}{
		"good arn": {
			inputTopic: Topic{
				ARN:  mockGoodARN,
				App:  mockApp,
				Env:  mockEnv,
				Wkld: mockSvc,
			},
			wanted: "app-env-svc-topic",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			name, err := tc.inputTopic.ID()
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, name)
			}
		})
	}
}
