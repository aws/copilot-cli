// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package elbv2

import (
	"context"
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
	mockARN1 := "mockListenerRuleARN1"
	mockARN2 := "mockListenerRuleARN2"
	testCases := map[string]struct {
		setUpMock   func(m *mocks.Mockapi)
		inARNs      []string
		wanted      []string
		wantedError error
	}{
		"fail to describe rules": {
			inARNs: []string{mockARN1},
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN1}),
				}).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get listener rule for mockListenerRuleARN1: some error"),
		},
		"cannot find listener rule": {
			inARNs: []string{mockARN1},
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN1}),
				}).Return(&elbv2.DescribeRulesOutput{}, nil)
			},
			wantedError: fmt.Errorf("cannot find listener rule mockListenerRuleARN1"),
		},
		"success": {
			inARNs: []string{mockARN1},
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN1}),
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
		"success in case of multiple rules": {
			inARNs: []string{mockARN1, mockARN2},
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRules(&elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN1, mockARN2}),
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
						{
							Conditions: []*elbv2.RuleCondition{
								{
									Field:  aws.String("path-pattern"),
									Values: []*string{aws.String("/*")},
								},
								{
									Field:  aws.String("host-header"),
									Values: aws.StringSlice([]string{"v1.copilot.com", "v1.archer.com"}),
									HostHeaderConfig: &elbv2.HostHeaderConditionConfig{
										Values: aws.StringSlice([]string{"v1.copilot.com", "v1.archer.com"}),
									},
								},
							},
						},
					},
				}, nil)
			},
			wanted: []string{"archer.com", "copilot.com", "v1.archer.com", "v1.copilot.com"},
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

			got, err := elbv2Client.ListenerRulesHostHeaders(tc.inARNs)

			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}

func TestELBV2_DescribeRule(t *testing.T) {
	mockARN := "mockListenerRuleARN"
	testCases := map[string]struct {
		setUpMock func(m *mocks.Mockapi)

		expectedErr string
		expected    Rule
	}{
		"fail to describe rules": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRulesWithContext(gomock.Any(), &elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(nil, errors.New("some error"))
			},
			expectedErr: "some error",
		},
		"cannot find listener rule": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRulesWithContext(gomock.Any(), &elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(&elbv2.DescribeRulesOutput{}, nil)
			},
			expectedErr: `not found`,
		},
		"success": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeRulesWithContext(gomock.Any(), &elbv2.DescribeRulesInput{
					RuleArns: aws.StringSlice([]string{mockARN}),
				}).Return(&elbv2.DescribeRulesOutput{
					Rules: []*elbv2.Rule{
						{
							RuleArn: aws.String(mockARN),
						},
					},
				}, nil)
			},
			expected: Rule(elbv2.Rule{
				RuleArn: aws.String(mockARN),
			}),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPI := mocks.NewMockapi(ctrl)
			tc.setUpMock(mockAPI)

			elbv2Client := ELBV2{
				client: mockAPI,
			}

			actual, err := elbv2Client.DescribeRule(context.Background(), mockARN)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestELBV2Rule_HasRedirectAction(t *testing.T) {
	testCases := map[string]struct {
		rule     Rule
		expected bool
	}{
		"rule doesn't have redirect": {
			rule: Rule(elbv2.Rule{
				Actions: []*elbv2.Action{
					{
						Type: aws.String(elbv2.ActionTypeEnumForward),
					},
				},
			}),
		},
		"rule has redirect": {
			rule: Rule(elbv2.Rule{
				Actions: []*elbv2.Action{
					{
						Type: aws.String(elbv2.ActionTypeEnumRedirect),
					},
				},
			}),
			expected: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.rule.HasRedirectAction())
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

func TestELBV2_LoadBalancer(t *testing.T) {
	mockOutput := &elbv2.DescribeLoadBalancersOutput{
		LoadBalancers: []*elbv2.LoadBalancer{
			{
				LoadBalancerArn:       aws.String("mockLBARN"),
				LoadBalancerName:      aws.String("mockLBName"),
				DNSName:               aws.String("mockDNSName"),
				Scheme:                aws.String("internet-facing"),
				CanonicalHostedZoneId: aws.String("mockHostedZoneID"),
				SecurityGroups:        aws.StringSlice([]string{"sg1", "sg2"}),
			},
		},
	}
	testCases := map[string]struct {
		setUpMock func(m *mocks.Mockapi)
		mockID    string

		expectedErr string
		expectedLB  *LoadBalancer
	}{
		"successfully return LB info from name": {
			mockID: "loadBalancerName",
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
					Names: []*string{aws.String("loadBalancerName")},
				}).Return(mockOutput, nil)
				m.EXPECT().DescribeListeners(&elbv2.DescribeListenersInput{
					LoadBalancerArn: aws.String("mockLBARN"),
				}).Return(&elbv2.DescribeListenersOutput{
					Listeners: []*elbv2.Listener{
						{
							ListenerArn: aws.String("mockListenerARN"),
							Port:        aws.Int64(80),
							Protocol:    aws.String("http"),
						},
					},
					NextMarker: nil,
				}, nil)
			},

			expectedLB: &LoadBalancer{
				ARN:          "mockLBARN",
				Name:         "mockLBName",
				DNSName:      "mockDNSName",
				Scheme:       "internet-facing",
				HostedZoneID: "mockHostedZoneID",
				Listeners: []Listener{
					{
						ARN:      "mockListenerARN",
						Port:     80,
						Protocol: "http",
					},
				},
				SecurityGroups: []string{"sg1", "sg2"},
			},
		},
		"successfully return LB info from ARN": {
			mockID: "arn:aws:elasticloadbalancing:us-west-2:123594734248:loadbalancer/app/ALBForImport/8db123b49az6de94",
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
					LoadBalancerArns: []*string{aws.String("arn:aws:elasticloadbalancing:us-west-2:123594734248:loadbalancer/app/ALBForImport/8db123b49az6de94")}}).Return(mockOutput, nil)
				m.EXPECT().DescribeListeners(&elbv2.DescribeListenersInput{
					LoadBalancerArn: aws.String("mockLBARN"),
				}).Return(&elbv2.DescribeListenersOutput{
					Listeners: []*elbv2.Listener{
						{
							ListenerArn: aws.String("mockListenerARN"),
							Port:        aws.Int64(80),
							Protocol:    aws.String("http"),
						},
					},
					NextMarker: nil,
				}, nil)
			},
			expectedLB: &LoadBalancer{
				ARN:          "mockLBARN",
				Name:         "mockLBName",
				DNSName:      "mockDNSName",
				Scheme:       "internet-facing",
				HostedZoneID: "mockHostedZoneID",
				Listeners: []Listener{
					{
						ARN:      "mockListenerARN",
						Port:     80,
						Protocol: "http",
					},
				},
				SecurityGroups: []string{"sg1", "sg2"},
			},
		},
		"error if describe LB call fails": {
			mockID: "loadBalancerName",
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
					Names: []*string{aws.String("loadBalancerName")}}).Return(nil, errors.New("some error"))
			},
			expectedErr: `describe load balancer "loadBalancerName": some error`,
		},
		"error if no load balancers returned": {
			mockID: "mockLBName",
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
					Names: []*string{aws.String("mockLBName")},
				}).Return(&elbv2.DescribeLoadBalancersOutput{
					LoadBalancers: []*elbv2.LoadBalancer{},
				}, nil)
			},
			expectedErr: `no load balancer "mockLBName" found`,
		},
		"error if describe listeners call fails": {
			mockID: "loadBalancerName",
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
					Names: []*string{aws.String("loadBalancerName")},
				}).Return(mockOutput, nil)
				m.EXPECT().DescribeListeners(&elbv2.DescribeListenersInput{
					LoadBalancerArn: aws.String("mockLBARN"),
				}).Return(nil, errors.New("some error"))
			},
			expectedErr: `describe listeners on load balancer "mockLBARN": some error`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPI := mocks.NewMockapi(ctrl)
			tc.setUpMock(mockAPI)

			elbv2Client := ELBV2{
				client: mockAPI,
			}

			actual, err := elbv2Client.LoadBalancer(tc.mockID)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.Equal(t, tc.expectedLB, actual)
			}
		})
	}
}

func TestELBV2_listeners(t *testing.T) {
	mockLBARN := aws.String("mockLoadBalancerARN")
	mockOutput := &elbv2.DescribeListenersOutput{
		Listeners: []*elbv2.Listener{
			{
				ListenerArn: aws.String("listenerARN1"),
				Port:        aws.Int64(80),
				Protocol:    aws.String("HTTP"),
			},
			{
				ListenerArn: aws.String("listenerARN2"),
				Port:        aws.Int64(443),
				Protocol:    aws.String("HTTPS"),
			},
		},
	}
	testCases := map[string]struct {
		setUpMock func(m *mocks.Mockapi)

		expectedErr       string
		expectedListeners []Listener
	}{
		"successfully return listeners from LB ARN": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeListeners(&elbv2.DescribeListenersInput{
					LoadBalancerArn: mockLBARN,
				}).Return(mockOutput, nil)
			},
			expectedListeners: []Listener{
				{
					ARN:      "listenerARN1",
					Port:     80,
					Protocol: "HTTP",
				},
				{
					ARN:      "listenerARN2",
					Port:     443,
					Protocol: "HTTPS",
				},
			},
		},
		"error if describe call fails": {
			setUpMock: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeListeners(&elbv2.DescribeListenersInput{
					LoadBalancerArn: mockLBARN,
				}).Return(nil, errors.New("some error"))
			},
			expectedErr: `describe listeners on load balancer "mockLoadBalancerARN": some error`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAPI := mocks.NewMockapi(ctrl)
			tc.setUpMock(mockAPI)

			elbv2Client := ELBV2{
				client: mockAPI,
			}

			actual, err := elbv2Client.listeners(aws.StringValue(mockLBARN))
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.Equal(t, tc.expectedListeners, actual)
			}
		})
	}
}
