// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatch provides a client to make API requests to Amazon CloudWatch Service.
package cloudwatch

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	compositeAlarmType = "Composite"
	metricAlarmType    = "Metric"
)

type cwClient interface {
	DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error)
	ListTagsForResource(input *cloudwatch.ListTagsForResourceInput) (*cloudwatch.ListTagsForResourceOutput, error)
}

// CloudWatch wraps an Amazon CloudWatch client.
type CloudWatch struct {
	client cwClient
}

// App contains basic info for an application.
type App struct {
	AppName     string
	EnvName     string
	ProjectName string
}

// AlarmStatus contains CloudWatch alarm status.
type AlarmStatus struct {
	Arn          string
	Name         string
	Reason       string
	Status       string
	Type         string
	UpdatedTimes int64
}

// New returns a CloudWatch struct configured against the input session.
func New(s *session.Session) *CloudWatch {
	return &CloudWatch{
		client: cloudwatch.New(s),
	}
}

// GetAlarms returns all CloudWatch alarms associated with the app.
func (c *CloudWatch) GetAlarms(app App) ([]AlarmStatus, error) {
	var alarmStatus []AlarmStatus
	var err error
	alarmResp := &cloudwatch.DescribeAlarmsOutput{}
	for {
		alarmResp, err = c.client.DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
			NextToken: alarmResp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("get CloudWatch alarms for app %s: %w", app.AppName, err)
		}
		for _, alarm := range alarmResp.CompositeAlarms {
			exist, err := c.validateAlarm(app, alarm.AlarmArn)
			if err != nil {
				return nil, fmt.Errorf("validate CloudWatch alarm %s: %w", *alarm.AlarmName, err)
			}
			if !exist {
				continue
			}
			alarmStatus = append(alarmStatus, AlarmStatus{
				Arn:          aws.StringValue(alarm.AlarmArn),
				Name:         aws.StringValue(alarm.AlarmName),
				Reason:       aws.StringValue(alarm.StateReason),
				Status:       aws.StringValue(alarm.StateValue),
				Type:         compositeAlarmType,
				UpdatedTimes: alarm.StateUpdatedTimestamp.Unix(),
			})
		}
		for _, alarm := range alarmResp.MetricAlarms {
			exist, err := c.validateAlarm(app, alarm.AlarmArn)
			if err != nil {
				return nil, fmt.Errorf("validate CloudWatch alarm %s, %w", *alarm.AlarmName, err)
			}
			if !exist {
				continue
			}
			alarmStatus = append(alarmStatus, AlarmStatus{
				Arn:          aws.StringValue(alarm.AlarmArn),
				Name:         aws.StringValue(alarm.AlarmName),
				Reason:       aws.StringValue(alarm.StateReason),
				Status:       aws.StringValue(alarm.StateValue),
				Type:         metricAlarmType,
				UpdatedTimes: alarm.StateUpdatedTimestamp.Unix(),
			})
		}
		if alarmResp.NextToken == nil {
			break
		}
	}
	return alarmStatus, nil
}

// validateAlarm validate if the CloudWatch alarm is associated with the given app.
func (c *CloudWatch) validateAlarm(app App, arn *string) (bool, error) {
	tagResp, err := c.client.ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
		ResourceARN: arn,
	})
	if err != nil {
		return false, err
	}
	m := map[string]map[string]bool{
		stack.AppTagKey: map[string]bool{
			app.AppName: false,
		},
		stack.EnvTagKey: map[string]bool{
			app.EnvName: false,
		},
		stack.ProjectTagKey: map[string]bool{
			app.ProjectName: false,
		},
	}
	for _, tag := range tagResp.Tags {
		if _, ok := m[*tag.Key][*tag.Value]; ok {
			m[*tag.Key][*tag.Value] = true
		}
	}
	for _, existMap := range m {
		for _, exist := range existMap {
			if !exist {
				return false, nil
			}
		}
	}
	return true, nil
}
