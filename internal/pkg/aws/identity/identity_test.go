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
	mockBadARN := "mockArn"
	mockARN := "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole"
	mockChinaARN := "arn:aws-cn:iam::1111:role/phonetool-test-CFNExecutionRole"
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
		"should return wrapped error if cannot parse the account arn": {
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetCallerIdentity(gomock.Any()).Return(&sts.GetCallerIdentityOutput{
					Account: &mockAccount,
					Arn:     &mockBadARN,
					UserId:  &mockUserID,
				}, nil)
			},
			wantErr: fmt.Errorf("parse caller arn: arn: invalid prefix"),
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
		"should return Identity in non standard partition": {
			callMock: func(m *mocks.Mockapi) {
				m.EXPECT().GetCallerIdentity(gomock.Any()).Return(&sts.GetCallerIdentityOutput{
					Account: &mockAccount,
					Arn:     &mockChinaARN,
					UserId:  &mockUserID,
				}, nil)
			},
			wantIdentity: Caller{
				Account:     mockAccount,
				RootUserARN: fmt.Sprintf("arn:aws-cn:iam::%s:root", mockAccount),
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

			if tc.wantErr != nil {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.wantIdentity, gotIdentity)
			}
		})
	}
}
