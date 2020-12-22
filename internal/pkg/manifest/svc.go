// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"
)

const (
	// LoadBalancedWebServiceType is a web service with a load balancer and Fargate as compute.
	LoadBalancedWebServiceType = "Load Balanced Web Service"
	// BackendServiceType is a service that cannot be accessed from the internet but can be reached from other services.
	BackendServiceType = "Backend Service"
)

var (
	errUnmarshalCountOpts = errors.New(`unmarshal "count" field to an integer or autoscaling configuration`)
)

// ServiceTypes are the supported service manifest types.
var ServiceTypes = []string{
	LoadBalancedWebServiceType,
	BackendServiceType,
}

// Range is a number range with maximum and minimum values.
type Range string

// Parse parses Range string and returns the min and max values.
// For example: 1-100 returns 1 and 100.
func (r Range) Parse() (min int, max int, err error) {
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

// ServiceImageWithPort represents a container image with an exposed port.
type ServiceImageWithPort struct {
	Image `yaml:",inline"`
	Port  *uint16 `yaml:"port"`
}

// Count is a custom type which supports unmarshaling yaml which
// can either be of type int or type Autoscaling.
type Count struct {
	Value       *int        // 0 is a valid value, so we want the default value to be nil.
	Autoscaling Autoscaling // Mutually exclusive with Value.
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Count
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v2) interface.
func (a *Count) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&a.Autoscaling); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !a.Autoscaling.IsEmpty() {
		return nil
	}

	if err := unmarshal(&a.Value); err != nil {
		return errUnmarshalCountOpts
	}
	return nil
}

// Autoscaling represents the configurable options for Auto Scaling.
type Autoscaling struct {
	Range        *Range         `yaml:"range"`
	CPU          *int           `yaml:"cpu_percentage"`
	Memory       *int           `yaml:"memory_percentage"`
	Requests     *int           `yaml:"requests"`
	ResponseTime *time.Duration `yaml:"response_time"`
}

// Options converts the service's Auto Scaling configuration into a format parsable
// by the templates pkg.
func (a *Autoscaling) Options() (*template.AutoscalingOpts, error) {
	if a.IsEmpty() {
		return nil, nil
	}
	min, max, err := a.Range.Parse()
	if err != nil {
		return nil, err
	}
	autoscalingOpts := template.AutoscalingOpts{
		MinCapacity: &min,
		MaxCapacity: &max,
	}
	if a.CPU != nil {
		autoscalingOpts.CPU = aws.Float64(float64(*a.CPU))
	}
	if a.Memory != nil {
		autoscalingOpts.Memory = aws.Float64(float64(*a.Memory))
	}
	if a.Requests != nil {
		autoscalingOpts.Requests = aws.Float64(float64(*a.Requests))
	}
	if a.ResponseTime != nil {
		responseTime := float64(*a.ResponseTime) / float64(time.Second)
		autoscalingOpts.ResponseTime = aws.Float64(responseTime)
	}
	return &autoscalingOpts, nil
}

// IsEmpty returns whether Autoscaling is empty.
func (a *Autoscaling) IsEmpty() bool {
	return a.Range == nil && a.CPU == nil && a.Memory == nil &&
		a.Requests == nil && a.ResponseTime == nil
}

func durationp(v time.Duration) *time.Duration {
	return &v
}

// ServiceDockerfileBuildRequired returns if the service container image should be built from local Dockerfile.
func ServiceDockerfileBuildRequired(svc interface{}) (bool, error) {
	return dockerfileBuildRequired("service", svc)
}
