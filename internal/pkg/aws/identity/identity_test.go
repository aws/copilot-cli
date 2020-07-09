// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestIdentity_Get(t *testing.T) {
	mockError := errors.New("error")
	mockARN := "mockArn"
	mockAccount := "123412341234"
	mockUserID := "mockUserID"

	testCases := map[string]struct {
		callMock func(m *mocks.Mockapi)

		wantIdentity Caller
		wantErr      error
	}{
		"should return wrapped error given error from STS GetCallerIdentity": {
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetCallerIdentity(gomock.Any()).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get caller identity: %w", mockError),
		},
		"should return Identity": {
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetCallerIdentity(gomock.Any()).Return(&sts.GetCallerIdentityOutput{
					Account: &mockAccount,
					Arn:     &mockARN,
					UserId:  &mockUserID,
				}, nil)
			},
			wantIdentity: Caller{
				Account:     mockAccount,
				RootUserARN: fmt.Sprintf("arn:aws:iam::%s:root", mockAccount),
				UserID:      mockUserID,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockapi(ctrl)

			sts := STS{
				client: mockClient,
			}

			tc.callMock(mockClient)

			gotIdentity, gotErr := sts.Get()

			require.Equal(t, tc.wantIdentity, gotIdentity)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
