// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type fakeHTTPClient struct {
	content []byte
	err     error
}

func (c *fakeHTTPClient) Get(url string) (resp *http.Response, err error) {
	if c.err != nil {
		return nil, c.err
	}
	r := httptest.NewRecorder()
	_, _ = r.Write(c.content)
	return r.Result(), nil
}

func TestSSMPluginCommand_StartSession(t *testing.T) {
	mockSession := &ecs.Session{
		SessionId:  aws.String("mockSessionID"),
		StreamUrl:  aws.String("mockStreamURL"),
		TokenValue: aws.String("mockTokenValue"),
	}
	var mockRunner *Mockrunner
	mockError := errors.New("some error")
	tests := map[string]struct {
		inSession   *ecs.Session
		setupMocks  func(controller *gomock.Controller)
		wantedError error
	}{
		"return error if fail to start session": {
			inSession: mockSession,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().InteractiveRun(ssmPluginBinaryName,
					[]string{`{"SessionId":"mockSessionID","StreamUrl":"mockStreamURL","TokenValue":"mockTokenValue"}`, "us-west-2", "StartSession"}).Return(mockError)
			},
			wantedError: fmt.Errorf("start session: some error"),
		},
		"success with no update and no install": {
			inSession: mockSession,
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = NewMockrunner(controller)
				mockRunner.EXPECT().InteractiveRun(ssmPluginBinaryName,
					[]string{`{"SessionId":"mockSessionID","StreamUrl":"mockStreamURL","TokenValue":"mockTokenValue"}`, "us-west-2", "StartSession"}).Return(nil)
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tc.setupMocks(ctrl)
			s := SSMPluginCommand{
				runner: mockRunner,
				sess: &session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				},
			}
			err := s.StartSession(tc.inSession)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
