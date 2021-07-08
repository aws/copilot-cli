// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"fmt"
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
)

// ServiceTypes are the supported service manifest types.
var ServiceTypes = []string{
	RequestDrivenWebServiceType,
	LoadBalancedWebServiceType,
	BackendServiceType,
}

// Range contains either a Range or a range configuration for Autoscaling ranges
type Range struct {
	Value       *IntRangeBand // Mutually exclusive with RangeConfig
	RangeConfig RangeConfig
}

// Parse extracts the min and max from RangeOpts
func (r Range) Parse() (min int, max int, err error) {
	if r.Value != nil && !r.RangeConfig.IsEmpty() {
		return 0, 0, errInvalidRangeOpts
	}

	if r.Value != nil {
		return r.Value.Parse()
	}

	return *r.RangeConfig.Min, *r.RangeConfig.Max, nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the RangeOpts
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (r *Range) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&r.RangeConfig); err != nil {
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

	if err := unmarshal(&r.Value); err != nil {
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
// This method implements the yaml.Unmarshaler (v2) interface.
func (c *Count) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&c.AdvancedCount); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if err := c.AdvancedCount.IsValid(); err != nil {
		return err
	}

	if !c.AdvancedCount.IsEmpty() {
		// Successfully unmarshalled AdvancedCount fields, return
		return nil
	}

	if err := unmarshal(&c.Value); err != nil {
		return errUnmarshalCountOpts
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

// AdvancedCount represents the configurable options for Auto Scaling as well as
// Capacity configuration (spot).
type AdvancedCount struct {
	Spot         *int           `yaml:"spot"` // mutually exclusive with other fields
	Range        *Range         `yaml:"range"`
	CPU          *int           `yaml:"cpu_percentage"`
	Memory       *int           `yaml:"memory_percentage"`
	Requests     *int           `yaml:"requests"`
	ResponseTime *time.Duration `yaml:"response_time"`
}

// IsEmpty returns whether AdvancedCount is empty.
func (a *AdvancedCount) IsEmpty() bool {
	return a.Range == nil && a.CPU == nil && a.Memory == nil &&
		a.Requests == nil && a.ResponseTime == nil && a.Spot == nil
}

// IgnoreRange returns whether desiredCount is specified on spot capacity
func (a *AdvancedCount) IgnoreRange() bool {
	return a.Spot != nil
}

func (a *AdvancedCount) hasAutoscaling() bool {
	return a.Range != nil || a.CPU != nil || a.Memory != nil ||
		a.Requests != nil || a.ResponseTime != nil
}

// IsValid checks to make sure Spot fields are compatible with other values in AdvancedCount
func (a *AdvancedCount) IsValid() error {
	// Spot translates to desiredCount; cannot specify with autoscaling
	if a.Spot != nil && a.hasAutoscaling() {
		return errInvalidAdvancedCount
	}

	// Range must be specified if using autoscaling
	if a.Range == nil && (a.CPU != nil || a.Memory != nil || a.Requests != nil || a.ResponseTime != nil) {
		return errInvalidAutoscaling
	}

	return nil
}

// ServiceDockerfileBuildRequired returns if the service container image should be built from local Dockerfile.
func ServiceDockerfileBuildRequired(svc interface{}) (bool, error) {
	return dockerfileBuildRequired("service", svc)
}

func IsTypeAService(t string) bool {
	for _, serviceType := range ServiceTypes {
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
	Path                *string        `yaml:"path"`
	SuccessCodes        *string        `yaml:"success_codes"`
	HealthyThreshold    *int64         `yaml:"healthy_threshold"`
	UnhealthyThreshold  *int64         `yaml:"unhealthy_threshold"`
	Timeout             *time.Duration `yaml:"timeout"`
	Interval            *time.Duration `yaml:"interval"`
	DeregistrationDelay *time.Duration `yaml:"deregistration_delay"`
	GracePeriod         *time.Duration `yaml:"grace_period"`
}

func (h *HTTPHealthCheckArgs) isEmpty() bool {
	return h.Path == nil && h.HealthyThreshold == nil && h.UnhealthyThreshold == nil &&
		h.Interval == nil && h.Timeout == nil && h.DeregistrationDelay == nil &&
		h.GracePeriod == nil
}

// HealthCheckArgsOrString is a custom type which supports unmarshaling yaml which
// can either be of type string or type HealthCheckArgs.
type HealthCheckArgsOrString struct {
	HealthCheckPath *string
	HealthCheckArgs HTTPHealthCheckArgs
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the HealthCheckArgsOrString
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (hc *HealthCheckArgsOrString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&hc.HealthCheckArgs); err != nil {
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

	if err := unmarshal(&hc.HealthCheckPath); err != nil {
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
