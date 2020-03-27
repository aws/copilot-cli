// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatch/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCloudWatch_GetAlarms(t *testing.T) {
	const (
		appName     = "mockApp"
		envName     = "mockEnv"
		projectName = "mockProject"
	)
	mockTime, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05+00:00")
	mockError := errors.New("some error")
	testCases := map[string]struct {
		mockcwClient func(m *mocks.MockcwClient)

		wantErr         error
		wantAlarmStatus []AlarmStatus
	}{
		"errors if failed to get CloudWatch alarms": {
			mockcwClient: func(m *mocks.MockcwClient) {
				m.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					NextToken: nil,
				}).Return(nil, mockError)
			},

			wantErr: fmt.Errorf("describe CloudWatch alarms: some error"),
		},
		"errors if failed to validate CloudWatch alarms": {
			mockcwClient: func(m *mocks.MockcwClient) {
				m.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					NextToken: nil,
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					NextToken: nil,
					CompositeAlarms: []*cloudwatch.CompositeAlarm{
						{
							AlarmArn:              aws.String("mockArn"),
							AlarmName:             aws.String("mockAlarmName"),
							StateReason:           aws.String("mockReason"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
				}, nil)
				m.EXPECT().ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
					ResourceARN: aws.String("mockArn"),
				}).Return(nil, mockError)
			},

			wantErr: fmt.Errorf("validate CloudWatch alarm mockAlarmName: some error"),
		},
		"success": {
			mockcwClient: func(m *mocks.MockcwClient) {
				m.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					NextToken: nil,
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					NextToken: nil,
					CompositeAlarms: []*cloudwatch.CompositeAlarm{
						{
							AlarmArn:              aws.String("mockArn1"),
							AlarmName:             aws.String("mockAlarmName1"),
							StateReason:           aws.String("mockReason"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
					MetricAlarms: []*cloudwatch.MetricAlarm{
						{
							AlarmArn:              aws.String("mockArn2"),
							AlarmName:             aws.String("mockAlarmName2"),
							StateReason:           aws.String("mockReason"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
				}, nil)
				m.EXPECT().ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
					ResourceARN: aws.String("mockArn1"),
				}).Return(&cloudwatch.ListTagsForResourceOutput{
					Tags: []*cloudwatch.Tag{
						{
							Key:   aws.String("ecs-project"),
							Value: aws.String("mockProject"),
						},
						{
							Key:   aws.String("ecs-application"),
							Value: aws.String("mockApp"),
						},
						{
							Key:   aws.String("ecs-environment"),
							Value: aws.String("mockEnv"),
						},
					},
				}, nil)
				m.EXPECT().ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
					ResourceARN: aws.String("mockArn2"),
				}).Return(&cloudwatch.ListTagsForResourceOutput{
					Tags: []*cloudwatch.Tag{
						{
							Key:   aws.String("ecs-project"),
							Value: aws.String("mockProject"),
						},
						{
							Key:   aws.String("ecs-application"),
							Value: aws.String("badMockApp"),
						},
						{
							Key:   aws.String("ecs-environment"),
							Value: aws.String("mockEnv"),
						},
					},
				}, nil)
			},

			wantAlarmStatus: []AlarmStatus{
				{
					Arn:          "mockArn1",
					Name:         "mockAlarmName1",
					Type:         "Composite",
					Reason:       "mockReason",
					Status:       "mockState",
					UpdatedTimes: mockTime.Unix(),
				},
			},
		},
		"success with pagination": {
			mockcwClient: func(m *mocks.MockcwClient) {
				m.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					NextToken: nil,
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					NextToken: aws.String("mockNextToken"),
					CompositeAlarms: []*cloudwatch.CompositeAlarm{
						{
							AlarmArn:              aws.String("mockArn1"),
							AlarmName:             aws.String("mockAlarmName1"),
							StateReason:           aws.String("mockReason"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
				}, nil)
				m.EXPECT().ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
					ResourceARN: aws.String("mockArn1"),
				}).Return(&cloudwatch.ListTagsForResourceOutput{
					Tags: []*cloudwatch.Tag{
						{
							Key:   aws.String("ecs-project"),
							Value: aws.String("mockProject"),
						},
						{
							Key:   aws.String("ecs-application"),
							Value: aws.String("mockApp"),
						},
						{
							Key:   aws.String("ecs-environment"),
							Value: aws.String("mockEnv"),
						},
					},
				}, nil)
				m.EXPECT().DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
					NextToken: aws.String("mockNextToken"),
				}).Return(&cloudwatch.DescribeAlarmsOutput{
					NextToken: nil,
					MetricAlarms: []*cloudwatch.MetricAlarm{
						{
							AlarmArn:              aws.String("mockArn2"),
							AlarmName:             aws.String("mockAlarmName2"),
							StateReason:           aws.String("mockReason"),
							StateValue:            aws.String("mockState"),
							StateUpdatedTimestamp: &mockTime,
						},
					},
				}, nil)
				m.EXPECT().ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
					ResourceARN: aws.String("mockArn2"),
				}).Return(&cloudwatch.ListTagsForResourceOutput{
					Tags: []*cloudwatch.Tag{
						{
							Key:   aws.String("ecs-project"),
							Value: aws.String("mockProject"),
						},
						{
							Key:   aws.String("ecs-application"),
							Value: aws.String("mockApp"),
						},
						{
							Key:   aws.String("ecs-environment"),
							Value: aws.String("mockEnv"),
						},
					},
				}, nil)
			},

			wantAlarmStatus: []AlarmStatus{
				{
					Arn:          "mockArn1",
					Name:         "mockAlarmName1",
					Type:         "Composite",
					Reason:       "mockReason",
					Status:       "mockState",
					UpdatedTimes: mockTime.Unix(),
				},
				{
					Arn:          "mockArn2",
					Name:         "mockAlarmName2",
					Type:         "Metric",
					Reason:       "mockReason",
					Status:       "mockState",
					UpdatedTimes: mockTime.Unix(),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcwClient := mocks.NewMockcwClient(ctrl)
			tc.mockcwClient(mockcwClient)

			cwSvc := CloudWatch{
				client: mockcwClient,
			}

			gotAlarmStatus, gotErr := cwSvc.GetAlarmsWithTags(map[string]string{
				"ecs-project":     projectName,
				"ecs-environment": envName,
				"ecs-application": appName,
			})

			if gotErr != nil {
				require.EqualError(t, tc.wantErr, gotErr.Error())
			} else {
				require.ElementsMatch(t, tc.wantAlarmStatus, gotAlarmStatus)
			}
		})

	}
}
