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

// HTTPOrBool holds advanced configuration for routing rule or a boolean switch.
type HTTPOrBool struct {
	HTTP
	Enabled *bool
}

func (r *HTTPOrBool) isEmpty() bool {
	return r.Enabled == nil && r.HTTP.IsEmpty()
}

// Disabled returns true if the routing rule configuration is explicitly disabled.
func (r *HTTPOrBool) Disabled() bool {
	return r.Enabled != nil && !aws.BoolValue(r.Enabled)
}

// UnmarshalYAML implements the yaml(v3) interface. It allows https routing rule to be specified as a
// bool or a struct alternately.
func (r *HTTPOrBool) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&r.HTTP); err != nil {
		switch err.(type) {
		case *yaml.TypeError:
			break
		default:
			return err
		}
	}

	if !r.HTTP.IsEmpty() {
		// Unmarshalled successfully to r.HTTP, unset r.Enabled, and return.
		r.Enabled = nil
		// this assignment lets us treat the main listener rule and additional listener rules equally
		// because we eliminate the need for TargetContainerCamelCase by assigning its value to TargetContainer.
		if r.TargetContainerCamelCase != nil && r.Main.TargetContainer == nil {
			r.Main.TargetContainer = r.TargetContainerCamelCase
			r.TargetContainerCamelCase = nil
		}
		return nil
	}

	if err := value.Decode(&r.Enabled); err != nil {
		return errors.New(`cannot marshal "http" field into bool or map`)
	}
	return nil
}

// HTTP holds options for application load balancer.
type HTTP struct {
	ImportedALB              *string       `yaml:"alb"`
	Main                     RoutingRule   `yaml:",inline"`
	TargetContainerCamelCase *string       `yaml:"targetContainer"` // Deprecated. Maintained for backwards compatibility, use [RoutingRule.TargetContainer] instead.
	AdditionalRoutingRules   []RoutingRule `yaml:"additional_rules"`
}

// RoutingRules returns main as well as additional routing rules as a list of RoutingRule.
func (cfg HTTP) RoutingRules() []RoutingRule {
	if cfg.Main.IsEmpty() {
		return nil
	}
	return append([]RoutingRule{cfg.Main}, cfg.AdditionalRoutingRules...)
}

// IsEmpty returns true if HTTP has empty configuration.
func (r *HTTP) IsEmpty() bool {
	return r.Main.IsEmpty() && r.TargetContainerCamelCase == nil && len(r.AdditionalRoutingRules) == 0
}

// RoutingRule holds listener rule configuration for ALB.
type RoutingRule struct {
	Path                *string                 `yaml:"path"`
	ProtocolVersion     *string                 `yaml:"version"`
	HealthCheck         HealthCheckArgsOrString `yaml:"healthcheck"`
	Stickiness          *bool                   `yaml:"stickiness"`
	Alias               Alias                   `yaml:"alias"`
	DeregistrationDelay *time.Duration          `yaml:"deregistration_delay"`
	// TargetContainer is the container load balancer routes traffic to.
	TargetContainer  *string `yaml:"target_container"`
	TargetPort       *uint16 `yaml:"target_port"`
	AllowedSourceIps []IPNet `yaml:"allowed_source_ips"`
	HostedZone       *string `yaml:"hosted_zone"`
	// RedirectToHTTPS configures a HTTP->HTTPS redirect. If nil, default to true.
	RedirectToHTTPS *bool `yaml:"redirect_to_https"`
}

// IsEmpty returns true if RoutingRule has empty configuration.
func (r *RoutingRule) IsEmpty() bool {
	return r.Path == nil && r.ProtocolVersion == nil && r.HealthCheck.IsZero() && r.Stickiness == nil && r.Alias.IsEmpty() &&
		r.DeregistrationDelay == nil && r.TargetContainer == nil && r.TargetPort == nil && r.AllowedSourceIps == nil &&
		r.HostedZone == nil && r.RedirectToHTTPS == nil
}

// HealthCheckPort returns the port a HealthCheck is set to for a RoutingRule.
func (r *RoutingRule) HealthCheckPort(mainContainerPort *uint16) uint16 {
	// healthCheckPort is defined by RoutingRule.HealthCheck.Port, with fallback on RoutingRule.TargetPort, then image.port.
	if r.HealthCheck.Advanced.Port != nil {
		return uint16(aws.IntValue(r.HealthCheck.Advanced.Port))
	}
	if r.TargetPort != nil {
		return aws.Uint16Value(r.TargetPort)
	}
	if mainContainerPort != nil {
		return aws.Uint16Value(mainContainerPort)
	}
	return 0
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
