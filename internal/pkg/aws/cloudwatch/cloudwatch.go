// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatch provides a client to make API requests to Amazon CloudWatch Service.
package cloudwatch

import (
	"fmt"

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

// GetAlarmsWithTags returns all the CloudWatch alarms that have the resource tags.
func (c *CloudWatch) GetAlarmsWithTags(tags map[string]string) ([]AlarmStatus, error) {
	var alarmStatus []AlarmStatus
	var err error
	alarmResp := &cloudwatch.DescribeAlarmsOutput{}
	for {
		alarmResp, err = c.client.DescribeAlarms(&cloudwatch.DescribeAlarmsInput{
			NextToken: alarmResp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe CloudWatch alarms: %w", err)
		}
		compositeAlarmStatus, err := c.filterCompositeAlarmsByTags(alarmResp.CompositeAlarms, tags)
		if err != nil {
			return nil, err
		}
		alarmStatus = append(alarmStatus, compositeAlarmStatus...)
		metricAlarmStatus, err := c.filterMetricAlarmsByTags(alarmResp.MetricAlarms, tags)
		if err != nil {
			return nil, err
		}
		alarmStatus = append(alarmStatus, metricAlarmStatus...)
		if alarmResp.NextToken == nil {
			break
		}
	}
	return alarmStatus, nil
}

func (c *CloudWatch) filterAlarmsByTags(alarms []AlarmStatus, tags map[string]string) ([]AlarmStatus, error) {
	var alarmStatusList []AlarmStatus
	for _, alarm := range alarms {
		exist, err := c.isAlarmTagged(alarm.Arn, tags)
		if err != nil {
			return nil, fmt.Errorf("validate CloudWatch alarm %s: %w", alarm.Name, err)
		}
		if !exist {
			continue
		}
		alarmStatusList = append(alarmStatusList, alarm)
	}
	return alarmStatusList, nil
}

func (c *CloudWatch) filterCompositeAlarmsByTags(alarms []*cloudwatch.CompositeAlarm, tags map[string]string) ([]AlarmStatus, error) {
	var alarmStatusList []AlarmStatus
	for _, alarm := range alarms {
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(alarm.AlarmArn),
			Name:         aws.StringValue(alarm.AlarmName),
			Reason:       aws.StringValue(alarm.StateReason),
			Status:       aws.StringValue(alarm.StateValue),
			Type:         compositeAlarmType,
			UpdatedTimes: alarm.StateUpdatedTimestamp.Unix(),
		})
	}
	return c.filterAlarmsByTags(alarmStatusList, tags)
}

func (c *CloudWatch) filterMetricAlarmsByTags(alarms []*cloudwatch.MetricAlarm, tags map[string]string) ([]AlarmStatus, error) {
	var alarmStatusList []AlarmStatus
	for _, alarm := range alarms {
		alarmStatusList = append(alarmStatusList, AlarmStatus{
			Arn:          aws.StringValue(alarm.AlarmArn),
			Name:         aws.StringValue(alarm.AlarmName),
			Reason:       aws.StringValue(alarm.StateReason),
			Status:       aws.StringValue(alarm.StateValue),
			Type:         metricAlarmType,
			UpdatedTimes: alarm.StateUpdatedTimestamp.Unix(),
		})
	}
	return c.filterAlarmsByTags(alarmStatusList, tags)
}

// isAlarmTagged validate if the CloudWatch alarm has the given tags.
func (c *CloudWatch) isAlarmTagged(alarmARN string, tags map[string]string) (bool, error) {
	tagResp, err := c.client.ListTagsForResource(&cloudwatch.ListTagsForResourceInput{
		ResourceARN: aws.String(alarmARN),
	})
	if err != nil {
		return false, err
	}
	m := make(map[string]map[string]bool)
	for k, v := range tags {
		m[k] = make(map[string]bool)
		m[k][v] = false
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
