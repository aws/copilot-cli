// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package elbv2

import (
	"errors"
	"fmt"
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

			got, err := elbv2Client.TargetsHealth(tc.targetGroupARN)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedOut, got)
			}
		})
	}
}

func TestELBV2_ListenerRuleHostHeaders(t *testing.T) {
	mockARN := "mockListenerRuleARN"
	testCases := map[string]struct {
		setUpMock func(m *mocks.Mockapi)

		wanted      []string
		wantedError error
	}{
		"fail to describe rules": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get listener rule for mockListenerRuleARN: some error"),
		},
		"cannot find listener rule": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(&elbv2.DescribeRulesOutput{}, nil)
			},
			wantedError: fmt.Errorf("cannot find listener rule mockListenerRuleARN"),
		},
		"success": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(&elbv2.DescribeRulesOutput{
					Rules: []*elbv2.Rule{
						{
							Conditions: []*elbv2.RuleCondition{
								{
									Field:  aws.String("path-pattern"),
									Values: []*string{aws.String("/*")},
								},
								{
									Field:  aws.String("host-header"),
									Values: aws.StringSlice([]string{"copilot.com", "archer.com"}),
									HostHeaderConfig: &elbv2.HostHeaderConditionConfig{
										Values: aws.StringSlice([]string{"copilot.com", "archer.com"}),
									},
								},
							},
						},
					},
				}, nil)
			},
			wanted: []string{"archer.com", "copilot.com"},
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

			got, err := elbv2Client.ListenerRuleHostHeaders(mockARN)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}

func TestTargetHealth_HealthStatus(t *testing.T) {
	testCases := map[string]struct {
		inTargetHealth *TargetHealth

		wanted *HealthStatus
	}{
		"unhealthy status with all params": {
			inTargetHealth: &TargetHealth{
				Target: &elbv2.TargetDescription{
					Id: aws.String("42.42.42.42"),
				},
				TargetHealth: &elbv2.TargetHealth{
					State:       aws.String("unhealthy"),
					Description: aws.String("some description"),
					Reason:      aws.String("some reason"),
				},
			},
			wanted: &HealthStatus{
				TargetID:          "42.42.42.42",
				HealthState:       "unhealthy",
				HealthDescription: "some description",
				HealthReason:      "some reason",
			},
		},
		"healthy status with description and reason empty": {
			inTargetHealth: &TargetHealth{
				Target: &elbv2.TargetDescription{
					Id: aws.String("24.24.24.24"),
				},
				TargetHealth: &elbv2.TargetHealth{
					State: aws.String("healthy"),
				},
			},
			wanted: &HealthStatus{
				TargetID:          "24.24.24.24",
				HealthState:       "healthy",
				HealthDescription: "",
				HealthReason:      "",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := tc.inTargetHealth.HealthStatus()
			require.Equal(t, got, tc.wanted)
		})
	}
}
