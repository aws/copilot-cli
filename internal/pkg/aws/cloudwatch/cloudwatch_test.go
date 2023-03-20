// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch/mocks"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type cloudWatchMocks struct {
	cw *mocks.Mockapi
	rg *mocks.MockresourceGetter
}

func TestCloudWatch_AlarmsWithTags(t *testing.T) {
	const (
		appName      = "mockApp"
		mockAlarmArn = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName"
	)
	mockError := errors.New("some error")
	mockTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	testTags := map[string]string{
		"copilot-application": appName,
	}

	testCases := map[string]struct {
		setupMocks func(m cloudWatchMocks)

		wantErr         error
		wantAlarmStatus []AlarmStatus
	}{
		"errors if failed to search resources": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return(nil, mockError)
			},

			wantErr: mockError,
		},
		"errors if failed to get alarm names because of invalid ARN": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{{ARN: "badArn"}}, nil)
			},

			wantErr: fmt.Errorf("parse alarm ARN badArn: arn: invalid prefix"),
		},
		"errors if failed to get alarm names because of bad ARN resource": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{{ARN: "arn:aws:cloudwatch:us-west-2:1234567890:alarm:badAlarm:Names"}}, nil)
			},

			wantErr: fmt.Errorf("unknown ARN resource format alarm:badAlarm:Names"),
		},
		"errors if failed to describe CloudWatch alarms": {
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{{ARN: mockAlarmArn}}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{"mockAlarmName"}),
					}).Return(nil, mockError),
				)
			},

			wantErr: fmt.Errorf("describe CloudWatch alarms: some error"),
		},
		"return if no alarms found": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{}, nil)
			},

			wantAlarmStatus: nil,
		},
		"should invoke DescribeAlarms on alarms that have matching tags": {
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]*rg.Resource{{ARN: mockAlarmArn}}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{"mockAlarmName"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:              aws.String(mockAlarmArn),
								AlarmName:             aws.String("mockAlarmName"),
								ComparisonOperator:    aws.String(cloudwatch.ComparisonOperatorGreaterThanOrEqualToThreshold),
								EvaluationPeriods:     aws.Int64(int64(300)),
								Period:                aws.Int64(int64(5)),
								Threshold:             aws.Float64(float64(70)),
								MetricName:            aws.String("mockMetricName"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
						CompositeAlarms: nil,
					}, nil))
			},
			wantAlarmStatus: []AlarmStatus{
				{
					Arn:          mockAlarmArn,
					Name:         "mockAlarmName",
					Type:         "Metric",
					Condition:    "mockMetricName ≥ 70.00 for 300 datapoints within 25 minutes",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				}},
		}}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcwClient := mocks.NewMockapi(ctrl)
			mockrgClient := mocks.NewMockresourceGetter(ctrl)
			mocks := cloudWatchMocks{
				cw: mockcwClient,
				rg: mockrgClient,
			}

			tc.setupMocks(mocks)

			cwSvc := CloudWatch{
				client:   mockcwClient,
				rgClient: mockrgClient,
			}

			gotAlarmStatus, gotErr := cwSvc.AlarmsWithTags(testTags)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.Equal(t, tc.wantAlarmStatus, gotAlarmStatus)
			}
		})

	}
}

func TestCloudWatch_AlarmStatuses(t *testing.T) {
	const (
		mockPrefix   = "some-prefix"
		mockName     = "mock-alarm-name"
		mockAlarmArn = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName"
		mockArn1     = mockAlarmArn + "1"
		mockArn2     = mockAlarmArn + "2"
	)
	mockError := errors.New("some error")
	mockTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")

	testCases := map[string]struct {
		setupMocks func(m cloudWatchMocks)
		in         DescribeAlarmOpts

		wantedErr           error
		wantedAlarmStatuses []AlarmStatus
	}{
		"errors if fail to describe alarms": {
			in: WithPrefix(mockPrefix),
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNamePrefix: aws.String(mockPrefix),
				}).Return(nil, mockError)
			},
			wantedErr: errors.New("describe CloudWatch alarms: some error"),
		},
		"return if no alarms with prefix found": {
			in: WithPrefix(mockPrefix),
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNamePrefix: aws.String(mockPrefix),
				}).Return(nil, nil)
			},
		},
		"success with prefix": {
			in: WithPrefix(mockPrefix),
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNamePrefix: aws.String(mockPrefix),
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					MetricAlarms: []*cloudwatch.MetricAlarm{
						{
							AlarmArn:              aws.String(mockAlarmArn),
							AlarmName:             aws.String(mockName),
							ComparisonOperator:    aws.String(cloudwatch.ComparisonOperatorGreaterThanOrEqualToThreshold),
							EvaluationPeriods:     aws.Int64(int64(300)),
							Period:                aws.Int64(int64(5)),
							Threshold:             aws.Float64(float64(70)),
							MetricName:            aws.String("mockMetricName"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
					CompositeAlarms: nil,
				}, nil)
			},
			wantedAlarmStatuses: []AlarmStatus{
				{
					Arn:          mockAlarmArn,
					Name:         mockName,
					Type:         "Metric",
					Condition:    "mockMetricName ≥ 70.00 for 300 datapoints within 25 minutes",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				}},
		},
		"success with static metric": {
			in: WithNames([]string{mockName}),
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{mockName}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:              aws.String(mockAlarmArn),
								AlarmName:             aws.String(mockName),
								ComparisonOperator:    aws.String(cloudwatch.ComparisonOperatorGreaterThanOrEqualToThreshold),
								EvaluationPeriods:     aws.Int64(int64(300)),
								Period:                aws.Int64(int64(5)),
								Threshold:             aws.Float64(float64(70)),
								MetricName:            aws.String("mockMetricName"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
				)
			},

			wantedAlarmStatuses: []AlarmStatus{
				{
					Arn:          mockAlarmArn,
					Name:         mockName,
					Type:         "Metric",
					Condition:    "mockMetricName ≥ 70.00 for 300 datapoints within 25 minutes",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
			},
		},
		"success with predictive metric": {
			in: WithNames([]string{mockName}),
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{mockName}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:           aws.String(mockArn1),
								AlarmName:          aws.String(mockName),
								ComparisonOperator: aws.String(cloudwatch.ComparisonOperatorLessThanLowerOrGreaterThanUpperThreshold),
								Metrics: []*cloudwatch.MetricDataQuery{
									{
										Id: aws.String("m1"),
										MetricStat: &cloudwatch.MetricStat{
											Period: aws.Int64(120),
											Metric: &cloudwatch.Metric{
												MetricName: aws.String("mockMetricName"),
											},
										},
										ReturnData: aws.Bool(true),
									},
									{
										Id:         aws.String("m2"),
										Expression: aws.String("ANOMALY_DETECTION_BAND(m1, 2)"),
										ReturnData: aws.Bool(true),
									},
								},
								ThresholdMetricId:     aws.String("m2"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
				)
			},

			wantedAlarmStatuses: []AlarmStatus{
				{
					Arn:          mockArn1,
					Name:         mockName,
					Type:         "Metric",
					Condition:    "-",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
			},
		},
		"success with predictive or dynamic metrics": {
			in: WithNames([]string{mockName}),
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{mockName}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:           aws.String(mockArn1),
								AlarmName:          aws.String(mockName),
								ComparisonOperator: aws.String(cloudwatch.ComparisonOperatorGreaterThanUpperThreshold),
								Metrics: []*cloudwatch.MetricDataQuery{
									{
										Id: aws.String("m1"),
										MetricStat: &cloudwatch.MetricStat{
											Period: aws.Int64(120),
											Metric: &cloudwatch.Metric{
												MetricName: aws.String("mockMetricName1"),
											},
										},
										ReturnData: aws.Bool(true),
									},
									{
										Id:         aws.String("m2"),
										Expression: aws.String("mockExpression"),
										ReturnData: aws.Bool(true),
									},
								},
								ThresholdMetricId:     aws.String("m2"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
							{
								AlarmArn:           aws.String(mockArn2),
								AlarmName:          aws.String("mockAlarmName2"),
								ComparisonOperator: aws.String(cloudwatch.ComparisonOperatorGreaterThanThreshold),
								Metrics: []*cloudwatch.MetricDataQuery{
									{
										Id: aws.String("m1"),
										MetricStat: &cloudwatch.MetricStat{
											Period: aws.Int64(120),
											Metric: &cloudwatch.Metric{
												MetricName: aws.String("mockMetricName2"),
											},
										},
										ReturnData: aws.Bool(true),
									},
									{
										Id:         aws.String("m2"),
										Expression: aws.String("mockExpression"),
										ReturnData: aws.Bool(true),
									},
								},
								ThresholdMetricId:     aws.String("m2"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
				)
			},

			wantedAlarmStatuses: []AlarmStatus{
				{
					Arn:          mockArn1,
					Name:         mockName,
					Type:         "Metric",
					Condition:    "-",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
				{
					Arn:          mockArn2,
					Name:         "mockAlarmName2",
					Type:         "Metric",
					Condition:    "-",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
			},
		},
		"success with pagination": {
			in: WithNames([]string{"mockAlarmName1", "mockAlarmName2"}),
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice([]string{"mockAlarmName1", "mockAlarmName2"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						NextToken: aws.String("mockNextToken"),
						CompositeAlarms: []*cloudwatch.CompositeAlarm{
							{
								AlarmArn:              aws.String(mockArn1),
								AlarmName:             aws.String("mockAlarmName1"),
								AlarmRule:             aws.String("mockAlarmRule"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
							nil,
						},
					}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  aws.String("mockNextToken"),
						AlarmNames: aws.StringSlice([]string{"mockAlarmName1", "mockAlarmName2"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:              aws.String(mockArn2),
								AlarmName:             aws.String("mockAlarmName2"),
								ComparisonOperator:    aws.String(cloudwatch.ComparisonOperatorLessThanThreshold),
								EvaluationPeriods:     aws.Int64(int64(60)),
								Period:                aws.Int64(int64(5)),
								DatapointsToAlarm:     aws.Int64(int64(3)),
								Threshold:             aws.Float64(float64(63)),
								MetricName:            aws.String("mockMetricName1"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
							nil,
						},
					}, nil),
				)
			},

			wantedAlarmStatuses: []AlarmStatus{
				{
					Arn:          mockArn1,
					Name:         "mockAlarmName1",
					Type:         "Composite",
					Condition:    "mockAlarmRule",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
				{
					Arn:          mockArn2,
					Name:         "mockAlarmName2",
					Type:         "Metric",
					Condition:    "mockMetricName1 < 63.00 for 3 datapoints within 5 minutes",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcwClient := mocks.NewMockapi(ctrl)
			mocks := cloudWatchMocks{
				cw: mockcwClient,
			}

			tc.setupMocks(mocks)

			cwSvc := CloudWatch{
				client: mockcwClient,
			}

			gotAlarmStatuses, gotErr := cwSvc.AlarmStatuses(tc.in)
			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedAlarmStatuses, gotAlarmStatuses)
			}
		})
	}
}

func TestCloudWatch_AlarmDescriptions(t *testing.T) {
	const (
		name1 = "mock-alarm-name1"
		name2 = "mock-alarm-name2"
		desc1 = "mock alarm description 1"
		desc2 = "mock alarm description 2"
	)
	mockNames := []string{name1, name2}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(m cloudWatchMocks)
		in         []string

		wantedErr               error
		wantedAlarmDescriptions []*AlarmDescription
	}{
		"errors if fail to describe alarms": {
			in: mockNames,
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNames: aws.StringSlice(mockNames),
				}).Return(nil, mockError)
			},
			wantedErr: errors.New("describe CloudWatch alarms: some error"),
		},
		"return if no alarms with names": {
			in: mockNames,
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNames: aws.StringSlice(mockNames),
				}).Return(nil, nil)
			},
		},
		"success": {
			in: mockNames,
			setupMocks: func(m cloudWatchMocks) {
				m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					AlarmNames: aws.StringSlice(mockNames),
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					MetricAlarms: []*cloudwatch.MetricAlarm{
						{
							AlarmName:        aws.String(name1),
							AlarmDescription: aws.String(desc1),
						},
						{
							AlarmName:        aws.String(name2),
							AlarmDescription: aws.String(desc2),
						},
					},
					CompositeAlarms: nil,
				}, nil)
			},
			wantedAlarmDescriptions: []*AlarmDescription{
				{
					Name:        name1,
					Description: desc1,
				},
				{
					Name:        name2,
					Description: desc2,
				},
			},
		},
	
		"success with pagination": {
			in: mockNames,
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						AlarmNames: aws.StringSlice(mockNames),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						NextToken: aws.String("mockNextToken"),
						CompositeAlarms: []*cloudwatch.CompositeAlarm{
							{
								AlarmName:             aws.String(name1),
								AlarmDescription:      aws.String(desc1),
							},
							nil,
						},
					}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  aws.String("mockNextToken"),
						AlarmNames: aws.StringSlice(mockNames),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmName:             aws.String(name2),
								AlarmDescription:      aws.String(desc2),
							},
							nil,
						},
					}, nil),
				)
			},

			wantedAlarmDescriptions: []*AlarmDescription{
				{
					Name:        name1,
					Description: desc1,
				},
				{
					Name:         name2,
					Description:  desc2,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcwClient := mocks.NewMockapi(ctrl)
			mocks := cloudWatchMocks{
				cw: mockcwClient,
			}

			tc.setupMocks(mocks)

			cwSvc := CloudWatch{
				client: mockcwClient,
			}

			gotAlarmDescriptions, gotErr := cwSvc.AlarmDescriptions(tc.in)
			if gotErr != nil {
				require.EqualError(t, gotErr, tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedAlarmDescriptions, gotAlarmDescriptions)
			}
		})
	}
}
