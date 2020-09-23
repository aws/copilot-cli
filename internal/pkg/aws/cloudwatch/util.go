// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatch provides a client to make API requests to Amazon CloudWatch Service.
package cloudwatch

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	anomalyDetectionBandExpression = "ANOMALY_DETECTION_BAND"
	defaultPeriod                  = 0
	defaultEvaluationPeriod        = 1

	// {metricTitle} {breachingRelationship} {threshold} for {datapointsCount} datapoints within {duration}
	fmtStaticMetricCondition = "%s %s %.2f for %d datapoints within %s"
	// {metricTitle} {breachingRelationship} the band (width: {bandWidth}) for {datapointsCount} datapoints within {duration}
	fmtPredictiveMetricCondition = "%s %s the band (width: %.2f) for %d datapoints within %s"
	// {metricTitle} {breachingRelationship} the dynamic threshold for {datapointsCount} datapoints within {duration}
	fmtDynamicMetricCondition = "%s %s the dynamic threshold for %d datapoints within %s"
	// {metricTitle} {breachingRelationship} the band for {datapointsCount} datapoints within {duration}
	fmtDynamicMetricConditionWithBand = "%s %s the band for %d datapoints within %s"
)

type alarmThresholdTypes int

const (
	predictive alarmThresholdTypes = iota
	dynamic
	static
)

func bandComparisonOperators() []string {
	return []string{
		cloudwatch.ComparisonOperatorGreaterThanUpperThreshold,
		cloudwatch.ComparisonOperatorLessThanLowerOrGreaterThanUpperThreshold,
		cloudwatch.ComparisonOperatorLessThanLowerThreshold,
	}
}

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

func (c comparisonOperator) isDynamicBand() bool {
	for _, operator := range bandComparisonOperators() {
		if string(c) == operator {
			return true
		}
	}
	return false
}

type metricAlarm cloudwatch.MetricAlarm

func (a metricAlarm) condition() string {
	thresholdType := a.alarmThresholdType()
	metricName := a.metricTitle()
	bandWidth := a.bandWidth(thresholdType)
	period := a.Period
	if period == nil {
		period = a.periodInAlarmMetrics()
	}
	evaluationPeriod := a.EvaluationPeriods
	if evaluationPeriod == nil {
		evaluationPeriod = aws.Int64(defaultEvaluationPeriod)
	}
	datapointsToAlarm := aws.Int64Value(a.DatapointsToAlarm)
	if datapointsToAlarm == 0 {
		datapointsToAlarm = aws.Int64Value(evaluationPeriod)
	}
	operator := comparisonOperator(aws.StringValue(a.ComparisonOperator))
	durationPeriod := time.Duration(aws.Int64Value(evaluationPeriod)*aws.Int64Value(period)) * time.Second
	switch thresholdType {
	case static:
		return fmt.Sprintf(fmtStaticMetricCondition, metricName, operator.humanString(), aws.Float64Value(a.Threshold), datapointsToAlarm,
			strings.TrimSpace(humanizeDuration(time.Now(), time.Now().Add(durationPeriod), "", "")))
	case dynamic:
		if operator.isDynamicBand() {
			return fmt.Sprintf(fmtDynamicMetricConditionWithBand, metricName, operator.humanString(), datapointsToAlarm,
				strings.TrimSpace(humanizeDuration(time.Now(), time.Now().Add(durationPeriod), "", "")))
		}
		return fmt.Sprintf(fmtDynamicMetricCondition, metricName, operator.humanString(), datapointsToAlarm,
			strings.TrimSpace(humanizeDuration(time.Now(), time.Now().Add(durationPeriod), "", "")))
	case predictive:
		return fmt.Sprintf(fmtPredictiveMetricCondition, metricName, operator.humanString(), aws.Float64Value(bandWidth), datapointsToAlarm,
			strings.TrimSpace(humanizeDuration(time.Now(), time.Now().Add(durationPeriod), "", "")))
	default:
		return ""
	}
}

func (a metricAlarm) bandWidth(thresholdType alarmThresholdTypes) *float64 {
	thresholdMetric := a.thresholdMetric()
	if thresholdType != predictive || thresholdMetric == nil {
		return nil
	}
	if thresholdMetric.Expression == nil {
		return nil
	}
	// Example for expression when it is predict type:
	// ANOMALY_DETECTION_BAND(m1, 2)
	bandWidthStr := strings.Split(strings.TrimSuffix(strings.TrimPrefix(
		strings.TrimPrefix(aws.StringValue(thresholdMetric.Expression), anomalyDetectionBandExpression), "("), ")"), ",")
	if len(bandWidthStr) != 2 {
		return nil
	}
	bandWidth, err := strconv.ParseFloat(strings.TrimSpace(bandWidthStr[1]), 64)
	if err != nil {
		return nil
	}
	return aws.Float64(bandWidth)
}

func (a metricAlarm) periodInAlarmMetrics() *int64 {
	for _, metric := range a.Metrics {
		if metric.MetricStat != nil {
			return metric.MetricStat.Period
		}
	}
	return aws.Int64(defaultPeriod)
}

func (a metricAlarm) metricTitle() string {
	if a.Metrics == nil || len(a.Metrics) == 0 {
		return aws.StringValue(a.MetricName)
	}
	metricTitle := ""
	for _, metricData := range a.Metrics {
		if aws.BoolValue(metricData.ReturnData) && aws.StringValue(metricData.Id) != aws.StringValue(a.ThresholdMetricId) {
			if metricData.MetricStat != nil && metricData.MetricStat.Metric != nil {
				metricTitle = aws.StringValue(metricData.MetricStat.Metric.MetricName)
			}
		}
	}
	return metricTitle
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
