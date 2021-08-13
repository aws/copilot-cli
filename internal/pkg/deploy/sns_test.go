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

func TestTopic_String(t *testing.T) {

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
			wanted:    "topic (svc)",
		},
		"bad arn format": {
			inputARN:    "bad arn",
			wantedError: errInvalidARN,
			inputApp:    mockApp,
			inputEnv:    mockEnv,
			inputWkld:   mockSvc,
		},
		"bad arn: non-copilot topic": {
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
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			topic, err := NewTopic(tc.inputARN, tc.inputApp, tc.inputEnv, tc.inputWkld)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, topic.String())
			}
		})
	}
}
