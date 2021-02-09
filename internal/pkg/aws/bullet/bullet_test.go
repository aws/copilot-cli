// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bullet

import (
	"testing"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/bullet/mocks"
	"github.com/aws/copilot-cli/internal/pkg/new-sdk-go/bullet"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestBullet_DescribeService(t *testing.T) {

	testCases := map[string]struct {
		serviceArn       string
		mockBulletClient func(m *mocks.Mockapi)

		wantErr error
		wantSvc Service
	}{
		"success": {
			serviceArn: "arn:aws:fusion:us-east-1:123456789123:service/service1/abc",
			mockBulletClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeService(&bullet.DescribeServiceInput{
					ServiceArn: aws.String("arn:aws:fusion:us-east-1:123456789123:service/service1/abc"),
				}).Return(&bullet.DescribeServiceOutput{
					Service: &bullet.Service{
						ServiceArn: aws.String("arn:aws:fusion:us-east-1:123456789123:service/service1/abc"),
					},
				}, nil)
			},
			wantSvc : Service{
				ServiceArn: "arn:aws:fusion:us-east-1:123456789123:service/service1/abc",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockBulletClient := mocks.NewMockapi(ctrl)
			tc.mockBulletClient(mockBulletClient)

			service := Bullet{
				client: mockBulletClient,
			}

			gotSvc, gotErr := service.DescribeService(tc.serviceArn)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.Equal(t, tc.wantSvc, gotSvc)
			}
		})
	}
}



