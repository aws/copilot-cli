package elbv2

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/elbv2"

	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2/mocks"
)

func TestELBV2_TargetsHealth(t *testing.T) {
	testCases := map[string]struct {
		targetGroupARN string

		setUpMock func(m *mocks.Mockapi)

		wantedOut   []*TargetHealth
		wantedError error
	}{
		"success": {
			targetGroupARN: "group-1",

			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
					TargetGroupArn: aws.String("group-1"),
				}).Return(&elbv2.DescribeTargetHealthOutput{
					TargetHealthDescriptions: []*elbv2.TargetHealthDescription{
						{
							HealthCheckPort: aws.String("80"),
							Target: &elbv2.TargetDescription{
								Id: aws.String("10.00.233"),
							},
							TargetHealth: &elbv2.TargetHealth{
								Description: aws.String("timed out"),
								Reason:      aws.String(elbv2.TargetHealthReasonEnumTargetTimeout),
								State:       aws.String(elbv2.TargetHealthStateEnumUnhealthy),
							},
						},
						{
							HealthCheckPort: aws.String("80"),
							Target: &elbv2.TargetDescription{
								Id: aws.String("10.00.322"),
							},
							TargetHealth: &elbv2.TargetHealth{
								State: aws.String(elbv2.TargetHealthStateEnumHealthy),
							},
						},
						{
							HealthCheckPort: aws.String("80"),
							Target: &elbv2.TargetDescription{
								Id: aws.String("10.00.332"),
							},
							TargetHealth: &elbv2.TargetHealth{
								Description: aws.String("mismatch"),
								Reason:      aws.String(elbv2.TargetHealthReasonEnumTargetResponseCodeMismatch),
								State:       aws.String(elbv2.TargetHealthStateEnumUnhealthy),
							},
						},
					},
				}, nil)
			},

			wantedOut: []*TargetHealth{
				{
					HealthCheckPort: aws.String("80"),
					Target: &elbv2.TargetDescription{
						Id: aws.String("10.00.233"),
					},
					TargetHealth: &elbv2.TargetHealth{
						Description: aws.String("timed out"),
						Reason:      aws.String(elbv2.TargetHealthReasonEnumTargetTimeout),
						State:       aws.String(elbv2.TargetHealthStateEnumUnhealthy),
					},
				},
				{
					HealthCheckPort: aws.String("80"),
					Target: &elbv2.TargetDescription{
						Id: aws.String("10.00.322"),
					},
					TargetHealth: &elbv2.TargetHealth{
						State: aws.String(elbv2.TargetHealthStateEnumHealthy),
					},
				},
				{
					HealthCheckPort: aws.String("80"),
					Target: &elbv2.TargetDescription{
						Id: aws.String("10.00.332"),
					},
					TargetHealth: &elbv2.TargetHealth{
						Description: aws.String("mismatch"),
						Reason:      aws.String(elbv2.TargetHealthReasonEnumTargetResponseCodeMismatch),
						State:       aws.String(elbv2.TargetHealthStateEnumUnhealthy),
					},
				},
			},
		},
		"failed to describe target health": {
			targetGroupARN: "group-1",

			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
					TargetGroupArn: aws.String("group-1"),
				}).Return(nil, errors.New("some error"))
			},

			wantedError: errors.New("describe target health for target group group-1: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPI := mocks.NewMockapi(ctrl)
			tc.setUpMock(mockAPI)

			elbv2Client := ELBV2{
				client: mockAPI,
			}

			got, err := elbv2Client.HealthStatus(tc.targetGroupARN)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedOut, got)
			}
		})
	}
}
