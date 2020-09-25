// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatch

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	anomalyDetectionBandExpression = "ANOMALY_DETECTION_BAND"
	// {metricTitle} {breachingRelationship} {threshold} for {datapointsCount} datapoints within {duration}
	fmtStaticMetricCondition = "%s %s %.2f for %d datapoints within %s"
)

type alarmThresholdTypes int

const (
	predictive alarmThresholdTypes = iota
	dynamic
	static
)

type comparisonOperator string

func (c comparisonOperator) humanString() string {
	switch c {
	case cloudwatch.ComparisonOperatorGreaterThanOrEqualToThreshold:
		return "≥"
	case cloudwatch.ComparisonOperatorGreaterThanThreshold:
		return ">"
	case cloudwatch.ComparisonOperatorLessThanThreshold:
		return "<"
	case cloudwatch.ComparisonOperatorLessThanOrEqualToThreshold:
		return "≤"
	case cloudwatch.ComparisonOperatorLessThanLowerOrGreaterThanUpperThreshold:
		return "outside"
	case cloudwatch.ComparisonOperatorLessThanLowerThreshold:
		return "<"
	case cloudwatch.ComparisonOperatorGreaterThanUpperThreshold:
		return ">"
	default:
		return ""
	}
}

type metricAlarm cloudwatch.MetricAlarm

func (a metricAlarm) condition() string {
	thresholdType := a.alarmThresholdType()
	metricName := aws.StringValue(a.MetricName)
	period := aws.Int64Value(a.Period)
	evaluationPeriod := aws.Int64Value(a.EvaluationPeriods)
	datapointsToAlarm := aws.Int64Value(a.DatapointsToAlarm)
	if datapointsToAlarm == 0 {
		// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cw-alarm.html#cfn-cloudwatch-alarm-datapointstoalarm
		datapointsToAlarm = evaluationPeriod
	}
	operator := comparisonOperator(aws.StringValue(a.ComparisonOperator))
	switch thresholdType {
	case static:
		return fmt.Sprintf(fmtStaticMetricCondition, metricName, operator.humanString(),
			aws.Float64Value(a.Threshold), datapointsToAlarm, humanizePeriod(evaluationPeriod, period))
	default:
		return "-"
	}
}

func (a metricAlarm) alarmThresholdType() alarmThresholdTypes {
	if a.ThresholdMetricId == nil || len(a.Metrics) < 2 {
		return static
	}
	thresholdMetric := a.thresholdMetric()
	if thresholdMetric != nil {
		if strings.HasPrefix(aws.StringValue(thresholdMetric.Expression), anomalyDetectionBandExpression) {
			return predictive
		}
	}
	return dynamic
}

func (a metricAlarm) thresholdMetric() *cloudwatch.MetricDataQuery {
	if a.ThresholdMetricId == nil {
		return nil
	}
	for _, m := range a.Metrics {
		if aws.StringValue(m.Id) == aws.StringValue(a.ThresholdMetricId) {
			return m
		}
	}
	return nil
}

func humanizePeriod(evaluationPeriod, period int64) string {
	durationPeriod := time.Duration(evaluationPeriod*period) * time.Second
	return strings.TrimSpace(humanizeDuration(time.Now(), time.Now().Add(durationPeriod), "", ""))
}
