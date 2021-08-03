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
		inputARN  string
		inputApp  string
		inputEnv  string
		inputWkld string

		wanted      string
		wantedError error
	}{
		"good arn": {
			inputARN:  mockGoodARN,
			inputApp:  mockApp,
			inputEnv:  mockEnv,
			inputWkld: mockSvc,
			wanted:    "topic",
		},
		"bad arn format": {
			inputARN:    "bad arn",
			wantedError: errInvalidARN,
		},
		"bad arn: for non-copilot topic": {
			inputARN:  mockBadARN,
			inputApp:  mockApp,
			inputEnv:  mockEnv,
			inputWkld: mockSvc,

			wantedError: errInvalidTopicARN,
		},
		"bad arn: arn for non-sns service": {
			inputARN:  "arn:aws:s3:::bucketname",
			inputApp:  mockApp,
			inputEnv:  mockEnv,
			inputWkld: mockSvc,

			wantedError: errInvalidARNService,
		},
		"bad arn: arn for copilot topic subscription": {

			inputARN:  mockGoodARN + ":12345-abcde-12345-abcde",
			inputApp:  mockApp,
			inputEnv:  mockEnv,
			inputWkld: mockSvc,

			wantedError: errInvalidARNService,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			topic, err := NewTopic(tc.inputARN, tc.inputApp, tc.inputEnv, tc.inputWkld)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, topic.Name())
			}
		})
	}
}

func TestTopic_ID(t *testing.T) {
	testCases := map[string]struct {
		inputARN  string
		inputApp  string
		inputEnv  string
		inputWkld string

		wanted      string
		wantedError error
	}{
		"good arn": {
			inputARN:  mockGoodARN,
			inputApp:  mockApp,
			inputEnv:  mockEnv,
			inputWkld: mockSvc,
			wanted:    "app-env-svc-topic",
		},
		"rejects improperly constructed ARN": {
			inputARN:    mockGoodARN,
			inputApp:    mockApp,
			inputEnv:    mockEnv,
			inputWkld:   "not-the-right-svc",
			wantedError: errInvalidTopicARN,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			topic, err := NewTopic(tc.inputARN, tc.inputApp, tc.inputEnv, tc.inputWkld)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, topic.ID())
			}
		})
	}
}
