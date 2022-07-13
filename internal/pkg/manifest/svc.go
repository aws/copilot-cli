// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

const (
	// LoadBalancedWebServiceType is a web service with a load balancer and Fargate as compute.
	LoadBalancedWebServiceType = "Load Balanced Web Service"
	// RequestDrivenWebServiceType is a Request-Driven Web Service managed by AppRunner
	RequestDrivenWebServiceType = "Request-Driven Web Service"
	// BackendServiceType is a service that cannot be accessed from the internet but can be reached from other services.
	BackendServiceType = "Backend Service"
	// WorkerServiceType is a worker service that manages the consumption of messages.
	WorkerServiceType = "Worker Service"
)

// ServiceTypes returns the list of supported service manifest types.
func ServiceTypes() []string {
	return []string{
		RequestDrivenWebServiceType,
		LoadBalancedWebServiceType,
		BackendServiceType,
		WorkerServiceType,
	}
}

// Range contains either a Range or a range configuration for Autoscaling ranges.
type Range struct {
	Value       *IntRangeBand // Mutually exclusive with RangeConfig
	RangeConfig RangeConfig
}

// IsEmpty returns whether Range is empty.
func (r *Range) IsEmpty() bool {
	return r.Value == nil && r.RangeConfig.IsEmpty()
}

// Parse extracts the min and max from RangeOpts.
func (r *Range) Parse() (min int, max int, err error) {
	if r.Value != nil {
		return r.Value.Parse()
	}
	return aws.IntValue(r.RangeConfig.Min), aws.IntValue(r.RangeConfig.Max), nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the RangeOpts
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (r *Range) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.RangeConfig); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.RangeConfig.IsEmpty() {
		// Unmarshaled successfully to r.RangeConfig, unset r.Range, and return.
		r.Value = nil
		return nil
	}

	if err := value.Decode(&r.Value); err != nil {
		return errUnmarshalRangeOpts
	}
	return nil
}

// IntRangeBand is a number range with maximum and minimum values.
type IntRangeBand string

// Parse parses Range string and returns the min and max values.
// For example: 1-100 returns 1 and 100.
func (r IntRangeBand) Parse() (min int, max int, err error) {
	minMax := strings.Split(string(r), "-")
	if len(minMax) != 2 {
		return 0, 0, fmt.Errorf("invalid range value %s. Should be in format of ${min}-${max}", string(r))
	}
	min, err = strconv.Atoi(minMax[0])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert minimum value %s to integer", minMax[0])
	}
	max, err = strconv.Atoi(minMax[1])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot convert maximum value %s to integer", minMax[1])
	}
	return min, max, nil
}

// RangeConfig containers a Min/Max and an optional SpotFrom field which
// specifies the number of services you want to start placing on spot. For
// example, if your range is 1-10 and `spot_from` is 5, up to 4 services will
// be placed on dedicated Fargate capacity, and then after that, any scaling
// event will place additioanl services on spot capacity.
type RangeConfig struct {
	Min      *int `yaml:"min"`
	Max      *int `yaml:"max"`
	SpotFrom *int `yaml:"spot_from"`
}

// IsEmpty returns whether RangeConfig is empty.
func (r *RangeConfig) IsEmpty() bool {
	return r.Min == nil && r.Max == nil && r.SpotFrom == nil
}

// Count is a custom type which supports unmarshaling yaml which
// can either be of type int or type AdvantedCount.
type Count struct {
	Value         *int          // 0 is a valid value, so we want the default value to be nil.
	AdvancedCount AdvancedCount // Mutually exclusive with Value.
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Count
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (c *Count) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&c.AdvancedCount); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !c.AdvancedCount.IsEmpty() {
		// Successfully unmarshalled AdvancedCount fields, return
		return nil
	}

	if err := value.Decode(&c.Value); err != nil {
		return errUnmarshalCountOpts
	}
	return nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the ScalingConfigOrT
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (r *ScalingConfigOrT[_]) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.ScalingConfig); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.ScalingConfig.IsEmpty() {
		// Successfully unmarshalled ScalingConfig fields, return
		return nil
	}

	if err := value.Decode(&r.Value); err != nil {
		return errors.New(`unable to unmarshal into int or composite-style map`)
	}
	return nil
}

// IsEmpty returns whether Count is empty.
func (c *Count) IsEmpty() bool {
	return c.Value == nil && c.AdvancedCount.IsEmpty()
}

// Desired returns the desiredCount to be set on the CFN template
func (c *Count) Desired() (*int, error) {
	if c.AdvancedCount.IsEmpty() {
		return c.Value, nil
	}

	if c.AdvancedCount.IgnoreRange() {
		return c.AdvancedCount.Spot, nil
	}
	min, _, err := c.AdvancedCount.Range.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse task count value %s: %w", aws.StringValue((*string)(c.AdvancedCount.Range.Value)), err)
	}
	return aws.Int(min), nil
}

// Percentage represents a valid percentage integer ranging from 0 to 100.
type Percentage int

// ScalingConfigOrT represents a resource that has autoscaling configurations or a generic value.
type ScalingConfigOrT[T ~int | time.Duration] struct {
	Value         *T
	ScalingConfig AdvancedScalingConfig[T] // mutually exclusive with Value
}

// AdvancedScalingConfig represents advanced configurable options for a scaling policy.
type AdvancedScalingConfig[T ~int | time.Duration] struct {
	Value    *T       `yaml:"value"`
	Cooldown Cooldown `yaml:"cooldown"`
}

// Cooldown represents the autoscaling cooldown of resources.
type Cooldown struct {
	ScaleInCooldown  *time.Duration `yaml:"in"`
	ScaleOutCooldown *time.Duration `yaml:"out"`
}

// AdvancedCount represents the configurable options for Auto Scaling as well as
// Capacity configuration (spot).
type AdvancedCount struct {
	Spot         *int                            `yaml:"spot"` // mutually exclusive with other fields
	Range        Range                           `yaml:"range"`
	Cooldown     Cooldown                        `yaml:"cooldown"`
	CPU          ScalingConfigOrT[Percentage]    `yaml:"cpu_percentage"`
	Memory       ScalingConfigOrT[Percentage]    `yaml:"memory_percentage"`
	Requests     ScalingConfigOrT[int]           `yaml:"requests"`
	ResponseTime ScalingConfigOrT[time.Duration] `yaml:"response_time"`
	QueueScaling QueueScaling                    `yaml:"queue_delay"`

	workloadType string
}

// IsEmpty returns whether ScalingConfigOrT is empty
func (r *ScalingConfigOrT[_]) IsEmpty() bool {
	return r.ScalingConfig.IsEmpty() && r.Value == nil
}

// IsEmpty returns whether AdvancedScalingConfig is empty
func (a *AdvancedScalingConfig[_]) IsEmpty() bool {
	return a.Cooldown.IsEmpty() && a.Value == nil
}

// IsEmpty returns whether Cooldown is empty
func (c *Cooldown) IsEmpty() bool {
	return c.ScaleInCooldown == nil && c.ScaleOutCooldown == nil
}

// IsEmpty returns whether AdvancedCount is empty.
func (a *AdvancedCount) IsEmpty() bool {
	return a.Range.IsEmpty() && a.CPU.IsEmpty() && a.Memory.IsEmpty() && a.Cooldown.IsEmpty() &&
		a.Requests.IsEmpty() && a.ResponseTime.IsEmpty() && a.Spot == nil && a.QueueScaling.IsEmpty()
}

// IgnoreRange returns whether desiredCount is specified on spot capacity
func (a *AdvancedCount) IgnoreRange() bool {
	return a.Spot != nil
}

func (a *AdvancedCount) hasAutoscaling() bool {
	return !a.Range.IsEmpty() || a.hasScalingFieldsSet()
}

func (a *AdvancedCount) validScalingFields() []string {
	switch a.workloadType {
	case LoadBalancedWebServiceType:
		return []string{"cpu_percentage", "memory_percentage", "requests", "response_time"}
	case BackendServiceType:
		return []string{"cpu_percentage", "memory_percentage", "requests", "response_time"}
	case WorkerServiceType:
		return []string{"cpu_percentage", "memory_percentage", "queue_delay"}
	default:
		return nil
	}
}

func (a *AdvancedCount) hasScalingFieldsSet() bool {
	switch a.workloadType {
	case LoadBalancedWebServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty()
	case BackendServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty()
	case WorkerServiceType:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.QueueScaling.IsEmpty()
	default:
		return !a.CPU.IsEmpty() || !a.Memory.IsEmpty() || !a.Requests.IsEmpty() || !a.ResponseTime.IsEmpty() || !a.QueueScaling.IsEmpty()
	}
}

func (a *AdvancedCount) getInvalidFieldsSet() []string {
	var invalidFields []string

	switch a.workloadType {
	case LoadBalancedWebServiceType:
		if !a.QueueScaling.IsEmpty() {
			invalidFields = append(invalidFields, "queue_delay")
		}
	case BackendServiceType:
		if !a.QueueScaling.IsEmpty() {
			invalidFields = append(invalidFields, "queue_delay")
		}
	case WorkerServiceType:
		if !a.Requests.IsEmpty() {
			invalidFields = append(invalidFields, "requests")
		}
		if !a.ResponseTime.IsEmpty() {
			invalidFields = append(invalidFields, "response_time")
		}
	}
	return invalidFields
}

func (a *AdvancedCount) unsetAutoscaling() {
	a.Range = Range{}
	a.Cooldown = Cooldown{}
	a.CPU = ScalingConfigOrT[Percentage]{}
	a.Memory = ScalingConfigOrT[Percentage]{}
	a.Requests = ScalingConfigOrT[int]{}
	a.ResponseTime = ScalingConfigOrT[time.Duration]{}
	a.QueueScaling = QueueScaling{}
}

// QueueScaling represents the configuration to scale a service based on a SQS queue.
type QueueScaling struct {
	AcceptableLatency *time.Duration `yaml:"acceptable_latency"`
	AvgProcessingTime *time.Duration `yaml:"msg_processing_time"`
	Cooldown          Cooldown       `yaml:"cooldown"`
}

// IsEmpty returns true if the QueueScaling is set.
func (qs *QueueScaling) IsEmpty() bool {
	return qs.AcceptableLatency == nil && qs.AvgProcessingTime == nil && qs.Cooldown.IsEmpty()
}

// AcceptableBacklogPerTask returns the total number of messages that each task can accumulate in the queue
// while maintaining the AcceptableLatency given the AvgProcessingTime.
func (qs *QueueScaling) AcceptableBacklogPerTask() (int, error) {
	if qs.IsEmpty() {
		return 0, errors.New(`"queue_delay" must be specified in order to calculate the acceptable backlog`)
	}
	v := math.Ceil(float64(*qs.AcceptableLatency) / float64(*qs.AvgProcessingTime))
	return int(v), nil
}

// IsTypeAService returns if manifest type is service.
func IsTypeAService(t string) bool {
	for _, serviceType := range ServiceTypes() {
		if t == serviceType {
			return true
		}
	}

	return false
}

// HTTPHealthCheckArgs holds the configuration to determine if the load balanced web service is healthy.
// These options are specifiable under the "healthcheck" field.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html.
type HTTPHealthCheckArgs struct {
	Path               *string        `yaml:"path"`
	Port               *int           `yaml:"port"`
	SuccessCodes       *string        `yaml:"success_codes"`
	HealthyThreshold   *int64         `yaml:"healthy_threshold"`
	UnhealthyThreshold *int64         `yaml:"unhealthy_threshold"`
	Timeout            *time.Duration `yaml:"timeout"`
	Interval           *time.Duration `yaml:"interval"`
	GracePeriod        *time.Duration `yaml:"grace_period"`
}

func (h *HTTPHealthCheckArgs) isEmpty() bool {
	return h.Path == nil && h.Port == nil && h.SuccessCodes == nil && h.HealthyThreshold == nil && h.UnhealthyThreshold == nil &&
		h.Interval == nil && h.Timeout == nil && h.GracePeriod == nil
}

// HealthCheckArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type HealthCheckArgs.
type HealthCheckArgsOrString struct {
	HealthCheckPath *string
	HealthCheckArgs HTTPHealthCheckArgs
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the HealthCheckArgsOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (hc *HealthCheckArgsOrString) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&hc.HealthCheckArgs); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !hc.HealthCheckArgs.isEmpty() {
		// Unmarshaled successfully to hc.HealthCheckArgs, reset hc.HealthCheckPath, and return.
		hc.HealthCheckPath = nil
		return nil
	}

	if err := value.Decode(&hc.HealthCheckPath); err != nil {
		return errUnmarshalHealthCheckArgs
	}
	return nil
}

// IsEmpty returns true if there are no health check configuration set.
func (hc *HealthCheckArgsOrString) IsEmpty() bool {
	if hc.HealthCheckPath != nil {
		return false
	}
	return hc.HealthCheckArgs.isEmpty()
}

// Path returns the default health check path if provided otherwise, returns the path from the advanced configuration.
func (hc *HealthCheckArgsOrString) Path() *string {
	if hc.HealthCheckPath != nil {
		return hc.HealthCheckPath
	}
	return hc.HealthCheckArgs.Path
}

// NLBHealthCheckArgs holds the configuration to determine if the network load balanced web service is healthy.
// These options are specifiable under the "healthcheck" field.
// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html.
type NLBHealthCheckArgs struct {
	Port               *int           `yaml:"port"`
	HealthyThreshold   *int64         `yaml:"healthy_threshold"`
	UnhealthyThreshold *int64         `yaml:"unhealthy_threshold"`
	Timeout            *time.Duration `yaml:"timeout"`
	Interval           *time.Duration `yaml:"interval"`
}

func (h *NLBHealthCheckArgs) isEmpty() bool {
	return h.Port == nil && h.HealthyThreshold == nil && h.UnhealthyThreshold == nil && h.Timeout == nil && h.Interval == nil
}

// ParsePortMapping parses port-protocol string into individual port and protocol strings.
// Valid examples: 2000/udp, or 2000.
func ParsePortMapping(s *string) (port *string, protocol *string, err error) {
	if s == nil {
		return nil, nil, nil
	}
	portProtocol := strings.Split(*s, "/")
	switch len(portProtocol) {
	case 1:
		return aws.String(portProtocol[0]), nil, nil
	case 2:
		return aws.String(portProtocol[0]), aws.String(portProtocol[1]), nil
	default:
		return nil, nil, fmt.Errorf("cannot parse port mapping from %s", *s)
	}
}
