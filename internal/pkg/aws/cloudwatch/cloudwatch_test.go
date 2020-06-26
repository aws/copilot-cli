// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"errors"
	"fmt"
	"testing"
	"time"

	cwmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch/mocks"
	rgmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type cloudWatchMocks struct {
	cw *cwmocks.Mockapi
	rg *rgmocks.MockResourceGroupsClient
}

func TestCloudWatch_GetAlarmsWithTags(t *testing.T) {
	const (
		appName      = "mockSvc"
		envName      = "mockEnv"
		projectName  = "mockApp"
		mockAlarmArn = "arn:aws:cloudwatch:us-west-2:1234567890:alarm:mockAlarmName"
		mockArn1     = mockAlarmArn + "1"
		mockArn2     = mockAlarmArn + "2"
	)
	mockTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	mockError := errors.New("some error")
	testTags := map[string]string{
		"copilot-application": projectName,
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
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{"badArn"}, nil)
			},

			wantErr: fmt.Errorf("parse alarm ARN badArn: arn: invalid prefix"),
		},
		"errors if failed to get alarm names because of bad ARN resource": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{"arn:aws:cloudwatch:us-west-2:1234567890:alarm:badAlarm:Names"}, nil)
			},

			wantErr: fmt.Errorf("cannot parse alarm ARN resource alarm:badAlarm:Names"),
		},
		"errors if failed to describe CloudWatch alarms": {
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{mockAlarmArn}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  nil,
						AlarmNames: aws.StringSlice([]string{"mockAlarmName"}),
					}).Return(nil, mockError),
				)
			},

			wantErr: fmt.Errorf("describe CloudWatch alarms: some error"),
		},
		"return an empty array if no alarms found": {
			setupMocks: func(m cloudWatchMocks) {
				m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{}, nil)
			},

			wantAlarmStatus: []AlarmStatus{},
		},
		"success": {
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{mockAlarmArn}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  nil,
						AlarmNames: aws.StringSlice([]string{"mockAlarmName"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						NextToken: nil,
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:              aws.String(mockAlarmArn),
								AlarmName:             aws.String("mockAlarmName"),
								StateReason:           aws.String("mockReason"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
				)
			},

			wantAlarmStatus: []AlarmStatus{
				{
					Arn:          mockAlarmArn,
					Name:         "mockAlarmName",
					Type:         "Metric",
					Reason:       "mockReason",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
			},
		},
		"success with pagination": {
			setupMocks: func(m cloudWatchMocks) {
				gomock.InOrder(
					m.rg.EXPECT().GetResourcesByTags(cloudwatchResourceType, gomock.Eq(testTags)).Return([]string{mockArn1, mockArn2}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  nil,
						AlarmNames: aws.StringSlice([]string{"mockAlarmName1", "mockAlarmName2"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						NextToken: aws.String("mockNextToken"),
						CompositeAlarms: []*cloudwatch.CompositeAlarm{
							{
								AlarmArn:              aws.String(mockArn1),
								AlarmName:             aws.String("mockAlarmName1"),
								StateReason:           aws.String("mockReason"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
					m.cw.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
						NextToken:  aws.String("mockNextToken"),
						AlarmNames: aws.StringSlice([]string{"mockAlarmName1", "mockAlarmName2"}),
					}).Return(&cloudwatch.DescribeAlarmsOutput{
						NextToken: nil,
						MetricAlarms: []*cloudwatch.MetricAlarm{
							{
								AlarmArn:              aws.String(mockArn2),
								AlarmName:             aws.String("mockAlarmName2"),
								StateReason:           aws.String("mockReason"),
								StateValue:            aws.String("mockState"),
								StateUpdatedTimestamp: &mockTime,
							},
						},
					}, nil),
				)
			},

			wantAlarmStatus: []AlarmStatus{
				{
					Arn:          mockArn1,
					Name:         "mockAlarmName1",
					Type:         "Composite",
					Reason:       "mockReason",
					Status:       "mockState",
					UpdatedTimes: mockTime,
				},
				{
					Arn:          mockArn2,
					Name:         "mockAlarmName2",
					Type:         "Metric",
					Reason:       "mockReason",
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

			mockcwClient := cwmocks.NewMockapi(ctrl)
			mockrgClient := rgmocks.NewMockResourceGroupsClient(ctrl)
			mocks := cloudWatchMocks{
				cw: mockcwClient,
				rg: mockrgClient,
			}

			tc.setupMocks(mocks)

			cwSvc := CloudWatch{
				mockcwClient,
				mockrgClient,
			}

			gotAlarmStatus, gotErr := cwSvc.GetAlarmsWithTags(testTags)

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantAlarmStatus, gotAlarmStatus)
			}
		})

	}
}
