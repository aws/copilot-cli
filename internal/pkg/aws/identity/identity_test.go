// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"errors"
	"fmt"
	"testing"

	awsmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/mocks"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/golang/mock/gomock"
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

		mockSTSAPI func(m *awsmocks.MockSTSAPI)

		wantIdentity Caller
		wantErr      error
	}{
		"should return wrapped error given error from STS GetCallerIdentity": {
			mockSTSAPI: func(m *awsmocks.MockSTSAPI) {
				m.EXPECT().GetCallerIdentity(&sts.GetCallerIdentityInput{}).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get caller identity: %w", mockError),
		},
		"should return Identity": {
			mockSTSAPI: func(m *awsmocks.MockSTSAPI) {
				m.EXPECT().GetCallerIdentity(&sts.GetCallerIdentityInput{}).Return(&sts.GetCallerIdentityOutput{
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
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSTSAPI := awsmocks.NewMockSTSAPI(ctrl)
			tc.mockSTSAPI(mockSTSAPI)

			service := Service{
				sts: mockSTSAPI,
			}

			gotIdentity, gotErr := service.Get()

			require.Equal(t, tc.wantIdentity, gotIdentity)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}
