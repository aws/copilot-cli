// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/require"
)

type mockSts struct {
	stsiface.STSAPI

	mockGetCallerIdentityFunc func(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)
}

func (ms mockSts) GetCallerIdentity(input *sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
	return ms.mockGetCallerIdentityFunc(input)
}

func TestGetCallerArn(t *testing.T) {
	mockError := errors.New("error")
	mockARN := "mockArn"
	mockAccount := "123412341234"
	mockUserID := "mockUserID"

	testCases := map[string]struct {
		mockGetCallerIdentityFunc func(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error)

		wantIdentity Caller
		wantErr      error
	}{
		"should return wrapped error given error from STS GetCallerIdentity": {
			mockGetCallerIdentityFunc: func(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
				return nil, mockError
			},
			wantErr: fmt.Errorf("get caller identity: %w", mockError),
		},
		"should return Identity": {
			mockGetCallerIdentityFunc: func(*sts.GetCallerIdentityInput) (*sts.GetCallerIdentityOutput, error) {
				return &sts.GetCallerIdentityOutput{
					Account: &mockAccount,
					Arn:     &mockARN,
					UserId:  &mockUserID,
				}, nil
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
			service := Service{
				mockSts{
					mockGetCallerIdentityFunc: tc.mockGetCallerIdentityFunc,
				},
			}

			gotIdentity, gotErr := service.Get()

			require.Equal(t, tc.wantIdentity, gotIdentity)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
