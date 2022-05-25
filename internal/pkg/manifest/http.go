// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v3"
)

// RoutingRuleConfigOrBool holds advanced configuration for routing rule or a boolean switch.
type RoutingRuleConfigOrBool struct {
	RoutingRuleConfiguration
	Enabled *bool
}

// Disabled returns true if the routing rule configuration is explicitly disabled.
func (r *RoutingRuleConfigOrBool) Disabled() bool {
	return r.Enabled != nil && !aws.BoolValue(r.Enabled)
}

// EmptyOrDisabled returns true if the routing rule configuration is not configured or is explicitly disabled.
func (r *RoutingRuleConfigOrBool) EmptyOrDisabled() bool {
	return r.Disabled() || r.isEmpty()
}

// UnmarshalYAML implements the yaml(v3) interface. It allows https routing rule to be specified as a
// bool or a struct alternately.
func (r *RoutingRuleConfigOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.RoutingRuleConfiguration); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.RoutingRuleConfiguration.isEmpty() {
		// Unmarshalled successfully to r.RoutingRuleConfiguration, unset r.Enabled, and return.
		r.Enabled = nil
		return nil
	}

	if err := value.Decode(&r.Enabled); err != nil {
		return errors.New(`cannot marshal "http" field into bool or map`)
	}
	return nil
}

// RoutingRuleConfiguration holds the path to route requests to the service.
type RoutingRuleConfiguration struct {
	Path                *string                 `yaml:"path"`
	ProtocolVersion     *string                 `yaml:"version"`
	HealthCheck         HealthCheckArgsOrString `yaml:"healthcheck"`
	Stickiness          *bool                   `yaml:"stickiness"`
	Alias               Alias                   `yaml:"alias"`
	DeregistrationDelay *time.Duration          `yaml:"deregistration_delay"`
	// TargetContainer is the container load balancer routes traffic to.
	TargetContainer          *string `yaml:"target_container"`
	TargetContainerCamelCase *string `yaml:"targetContainer"` // "targetContainerCamelCase" for backwards compatibility
	AllowedSourceIps         []IPNet `yaml:"allowed_source_ips"`
	HostedZone               *string `yaml:"hosted_zone"`
}

// GetTargetContainer returns the correct target container value, if set.
// Use this function instead of getting r.TargetContainer or r.TargetContainerCamelCase directly.
func (r *RoutingRuleConfiguration) GetTargetContainer() *string {
	if r.TargetContainer != nil {
		return r.TargetContainer
	}
	return r.TargetContainerCamelCase
}

func (r *RoutingRuleConfiguration) isEmpty() bool {
	return r.Path == nil && r.ProtocolVersion == nil && r.HealthCheck.IsEmpty() && r.Stickiness == nil && r.Alias.IsEmpty() &&
		r.DeregistrationDelay == nil && r.TargetContainer == nil && r.TargetContainerCamelCase == nil && r.AllowedSourceIps == nil &&
		r.HostedZone == nil
}

// IPNet represents an IP network string. For example: 10.1.0.0/16
type IPNet string

func ipNetP(s string) *IPNet {
	if s == "" {
		return nil
	}
	ip := IPNet(s)
	return &ip
}

// Alias is a custom type which supports unmarshaling "http.alias" yaml which
// can either be of type string or type slice of string.
type Alias stringSliceOrString

// IsEmpty returns empty if Alias is empty.
func (e *Alias) IsEmpty() bool {
	return e.String == nil && e.StringSlice == nil
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Alias
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (e *Alias) UnmarshalYAML(value *yaml.Node) error {
	if err := unmarshalYAMLToStringSliceOrString((*stringSliceOrString)(e), value); err != nil {
		return errUnmarshalAlias
	}
	return nil
}

// ToStringSlice converts an Alias to a slice of string using shell-style rules.
func (e *Alias) ToStringSlice() ([]string, error) {
	out, err := toStringSlice((*stringSliceOrString)(e))
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ToString converts an Alias to a string.
func (e *Alias) ToString() string {
	if e.String != nil {
		return aws.StringValue(e.String)
	}
	return strings.Join(e.StringSlice, ",")
}
