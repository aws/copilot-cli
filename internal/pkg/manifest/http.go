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

	if !r.RoutingRuleConfiguration.IsEmpty() {
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
	TargetPort               *uint16 `yaml:"target_port"`
	TargetContainerCamelCase *string `yaml:"targetContainer"` // "targetContainerCamelCase" for backwards compatibility
	AllowedSourceIps         []IPNet `yaml:"allowed_source_ips"`
	HostedZone               *string `yaml:"hosted_zone"`
	// RedirectToHTTPS configures a HTTP->HTTPS redirect. If nil, default to true.
	RedirectToHTTPS *bool `yaml:"redirect_to_https"`
}

// GetTargetContainer returns the correct target container value, if set.
// Use this function instead of getting r.TargetContainer or r.TargetContainerCamelCase directly.
func (r *RoutingRuleConfiguration) GetTargetContainer() *string {
	if r.TargetContainer != nil {
		return r.TargetContainer
	}
	return r.TargetContainerCamelCase
}

// IsEmpty returns true if RoutingRuleConfiguration has empty configuration.
func (r *RoutingRuleConfiguration) IsEmpty() bool {
	return r.Path == nil && r.ProtocolVersion == nil && r.HealthCheck.IsZero() && r.Stickiness == nil && r.Alias.IsEmpty() &&
		r.DeregistrationDelay == nil && r.TargetContainer == nil && r.TargetContainerCamelCase == nil && r.AllowedSourceIps == nil &&
		r.HostedZone == nil && r.RedirectToHTTPS == nil
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

// AdvancedAlias represents advanced alias configuration.
type AdvancedAlias struct {
	Alias      *string `yaml:"name"`
	HostedZone *string `yaml:"hosted_zone"`
}

// Alias is a custom type which supports unmarshaling "http.alias" yaml which
// can either be of type advancedAlias slice or type StringSliceOrString.
type Alias struct {
	AdvancedAliases     []AdvancedAlias
	StringSliceOrString StringSliceOrString
}

// HostedZones returns all the hosted zones.
func (a *Alias) HostedZones() []string {
	var hostedZones []string
	for _, alias := range a.AdvancedAliases {
		if alias.HostedZone != nil {
			hostedZones = append(hostedZones, *alias.HostedZone)
		}
	}
	return hostedZones
}

// IsEmpty returns empty if Alias is empty.
func (a *Alias) IsEmpty() bool {
	return len(a.AdvancedAliases) == 0 && a.StringSliceOrString.isEmpty()
}

// UnmarshalYAML overrides the default YAML unmarshaling logic for the Alias
// struct, allowing it to perform more complex unmarshaling behavior.
// This method implements the yaml.Unmarshaler (v3) interface.
func (a *Alias) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&a.AdvancedAliases); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if len(a.AdvancedAliases) != 0 {
		// Unmarshaled successfully to s.StringSlice, unset s.String, and return.
		a.StringSliceOrString = StringSliceOrString{}
		return nil
	}
	if err := a.StringSliceOrString.UnmarshalYAML(value); err != nil {
		return errUnmarshalAlias
	}
	return nil
}

// ToStringSlice converts an Alias to a slice of string.
func (a *Alias) ToStringSlice() ([]string, error) {
	if len(a.AdvancedAliases) == 0 {
		return a.StringSliceOrString.ToStringSlice(), nil
	}
	aliases := make([]string, len(a.AdvancedAliases))
	for i, advancedAlias := range a.AdvancedAliases {
		aliases[i] = aws.StringValue(advancedAlias.Alias)
	}
	return aliases, nil
}

// ToString converts an Alias to a string.
func (a *Alias) ToString() string {
	if len(a.AdvancedAliases) != 0 {
		aliases := make([]string, len(a.AdvancedAliases))
		for i, advancedAlias := range a.AdvancedAliases {
			aliases[i] = aws.StringValue(advancedAlias.Alias)
		}
		return strings.Join(aliases, ",")
	}
	if a.StringSliceOrString.String != nil {
		return aws.StringValue(a.StringSliceOrString.String)
	}
	return strings.Join(a.StringSliceOrString.StringSlice, ",")
}
