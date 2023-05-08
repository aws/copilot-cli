// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudfront"
)

var (
	errAZsNotEqual = errors.New("public subnets and private subnets do not span the same availability zones")

	minAZs = 2
)

// Validate returns nil if Environment is configured correctly.
func (e Environment) Validate() error {
	if err := e.EnvironmentConfig.validate(); err != nil {
		return err
	}
	return nil
}

// validate returns nil if EnvironmentConfig is configured correctly.
func (e EnvironmentConfig) validate() error {
	if err := e.Network.validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err := e.Observability.validate(); err != nil {
		return fmt.Errorf(`validate "observability": %w`, err)
	}
	if err := e.HTTPConfig.validate(); err != nil {
		return fmt.Errorf(`validate "http config": %w`, err)
	}
	if err := e.Network.VPC.SecurityGroupConfig.validate(); err != nil {
		return fmt.Errorf(`validate "security_group": %w`, err)
	}
	if err := e.CDNConfig.validate(); err != nil {
		return fmt.Errorf(`validate "cdn": %w`, err)
	}
	if e.IsPublicLBIngressRestrictedToCDN() && !e.CDNEnabled() {
		return errors.New("CDN must be enabled to limit security group ingress to CloudFront")
	}
	if e.CDNEnabled() {
		cdnCert := e.CDNConfig.Config.Certificate
		if e.HTTPConfig.Public.Certificates == nil {
			if cdnCert != nil && !aws.BoolValue(e.CDNConfig.Config.TerminateTLS) {
				return errors.New(`"cdn.terminate_tls" must be true if "cdn.certificate" is set without "http.public.certificates"`)
			}
		} else {
			if cdnCert == nil {
				return &errFieldMustBeSpecified{
					missingField:       "cdn.certificate",
					conditionalFields:  []string{"http.public.certificates", "cdn"},
					allMustBeSpecified: true,
				}
			}
		}
	}

	if e.HTTPConfig.Private.InternalALBSubnets != nil {
		if !e.Network.VPC.imported() {
			return errors.New("in order to specify internal ALB subnet placement, subnets must be imported")
		}
		if err := e.validateInternalALBSubnets(); err != nil {
			return err
		}
	}
	return nil
}

// validate returns nil if environmentNetworkConfig is configured correctly.
func (n environmentNetworkConfig) validate() error {
	if err := n.VPC.validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// validate returns nil if environmentVPCConfig is configured correctly.
func (cfg environmentVPCConfig) validate() error {
	if cfg.imported() && cfg.managedVPCCustomized() {
		return errors.New(`cannot import VPC resources (with "id" fields) and customize VPC resources (with "cidr" and "az" fields) at the same time`)
	}
	if err := cfg.Subnets.validate(); err != nil {
		return fmt.Errorf(`validate "subnets": %w`, err)
	}
	if cfg.imported() {
		if err := cfg.validateImportedVPC(); err != nil {
			return fmt.Errorf(`validate "subnets" for an imported VPC: %w`, err)
		}
	}
	if cfg.managedVPCCustomized() {
		if err := cfg.validateManagedVPC(); err != nil {
			return fmt.Errorf(`validate "subnets" for an adjusted VPC: %w`, err)
		}
	}
	if err := cfg.FlowLogs.validate(); err != nil {
		return fmt.Errorf(`validate vpc "flowlogs": %w`, err)
	}
	return nil
}

// validate returns nil if securityGroupRule has all the required parameters set.
func (cfg securityGroupRule) validate() error {
	if cfg.CidrIP == "" {
		return &errFieldMustBeSpecified{
			missingField: "cidr",
		}
	}
	if cfg.IpProtocol == "" {
		return &errFieldMustBeSpecified{
			missingField: "ip_protocol",
		}
	}
	return cfg.Ports.validate()
}

// validate if ports are set.
func (cfg portsConfig) validate() error {
	if cfg.IsEmpty() {
		return &errFieldMustBeSpecified{
			missingField: "ports",
		}
	}
	if cfg.Range == nil {
		return nil
	}
	if err := cfg.Range.validate(); err != nil {
		var targetErr *errInvalidRange
		if errors.As(err, &targetErr) {
			return &errInvalidRange{
				value:       aws.StringValue((*string)(cfg.Range)),
				validFormat: "${from_port}-${to_port}",
			}
		}
		return err
	}
	return nil
}

// validate returns nil if securityGroupConfig is configured correctly.
func (cfg securityGroupConfig) validate() error {
	for idx, ingress := range cfg.Ingress {
		if err := ingress.validate(); err != nil {
			return fmt.Errorf(`validate ingress[%d]: %w`, idx, err)
		}
	}
	for idx, egress := range cfg.Egress {
		if err := egress.validate(); err != nil {
			return fmt.Errorf(`validate egress[%d]: %w`, idx, err)
		}
	}
	return nil
}

func (cfg environmentVPCConfig) validateImportedVPC() error {
	for idx, subnet := range cfg.Subnets.Public {
		if aws.StringValue(subnet.SubnetID) == "" {
			return fmt.Errorf(`validate public[%d]: %w`, idx, &errFieldMustBeSpecified{
				missingField: "id",
			})
		}
	}
	for idx, subnet := range cfg.Subnets.Private {
		if aws.StringValue(subnet.SubnetID) == "" {
			return fmt.Errorf(`validate private[%d]: %w`, idx, &errFieldMustBeSpecified{
				missingField: "id",
			})
		}
	}
	switch {
	case len(cfg.Subnets.Private)+len(cfg.Subnets.Public) <= 0:
		return errors.New(`VPC must have subnets in order to proceed with environment creation`)
	case len(cfg.Subnets.Public) == 1:
		return errors.New(`validate "public": at least two public subnets must be imported to enable Load Balancing`)
	case len(cfg.Subnets.Private) == 1:
		return errors.New(`validate "private": at least two private subnets must be imported`)
	}
	return nil
}

func (cfg environmentVPCConfig) validateManagedVPC() error {
	var (
		publicAZs    = make(map[string]struct{})
		privateAZs   = make(map[string]struct{})
		publicCIDRs  = make(map[string]struct{})
		privateCIDRs = make(map[string]struct{})
	)
	var exists = struct{}{}
	for idx, subnet := range cfg.Subnets.Public {
		if aws.StringValue((*string)(subnet.CIDR)) == "" {
			return fmt.Errorf(`validate public[%d]: %w`, idx, &errFieldMustBeSpecified{
				missingField: "cidr",
			})
		}
		publicCIDRs[aws.StringValue((*string)(subnet.CIDR))] = exists
		if aws.StringValue(subnet.AZ) != "" {
			publicAZs[aws.StringValue(subnet.AZ)] = exists
		}
	}
	for idx, subnet := range cfg.Subnets.Private {
		if aws.StringValue((*string)(subnet.CIDR)) == "" {
			return fmt.Errorf(`validate private[%d]: %w`, idx, &errFieldMustBeSpecified{
				missingField: "cidr",
			})
		}
		privateCIDRs[aws.StringValue((*string)(subnet.CIDR))] = exists
		if aws.StringValue(subnet.AZ) != "" {
			privateAZs[aws.StringValue(subnet.AZ)] = exists
		}
	}
	// NOTE: the following are constraints on az:
	// 1. #az = 0, or #az = #public_subnets = #private_subnets.
	// 2. set(az_for_public) = set(az_for_private).
	// 3, If configured at all, the number of AZ must be >= 2.
	if !areSetsEqual(publicAZs, privateAZs) {
		return errAZsNotEqual
	}
	numAZs := len(publicAZs)
	if numAZs == 0 {
		return nil
	}
	if numAZs < minAZs {
		return fmt.Errorf(`require at least %d availability zones`, minAZs)
	}
	if len(publicCIDRs) != numAZs {
		return fmt.Errorf(`validate "public": number of public subnet CIDRs (%d) does not match number of AZs (%d)`, len(publicCIDRs), len(publicAZs))
	}
	if len(privateCIDRs) != numAZs {
		return fmt.Errorf(`validate "private": number of private subnet CIDRs (%d) does not match number of AZs (%d)`, len(privateCIDRs), len(publicAZs))
	}
	return nil
}

// validate returns nil if subnetsConfiguration is configured correctly.
func (cs subnetsConfiguration) validate() error {
	for idx, subnet := range cs.Public {
		if err := subnet.validate(); err != nil {
			return fmt.Errorf(`validate "public[%d]": %w`, idx, err)
		}
	}
	for idx, subnet := range cs.Private {
		if err := subnet.validate(); err != nil {
			return fmt.Errorf(`validate "private[%d]": %w`, idx, err)
		}
	}
	return nil
}

// validate returns nil if subnetConfiguration is configured correctly.
func (c subnetConfiguration) validate() error {
	if c.SubnetID != nil && c.CIDR != nil {
		return &errFieldMutualExclusive{
			firstField:  "id",
			secondField: "cidr",
			mustExist:   false,
		}
	}
	if c.SubnetID != nil && c.AZ != nil {
		return &errFieldMutualExclusive{
			firstField:  "id",
			secondField: "az",
			mustExist:   false,
		}
	}
	return nil
}

// validate is a no-op for VPCFlowLogsArgs.
func (fl VPCFlowLogsArgs) validate() error {
	return nil
}

// validate returns nil if environmentObservability is configured correctly.
func (o environmentObservability) validate() error {
	return nil
}

// validate returns nil if EnvironmentHTTPConfig is configured correctly.
func (cfg EnvironmentHTTPConfig) validate() error {
	if err := cfg.Public.validate(); err != nil {
		return fmt.Errorf(`validate "public": %w`, err)
	}
	if err := cfg.Private.validate(); err != nil {
		return fmt.Errorf(`validate "private": %w`, err)
	}
	return nil
}

// validate returns nil if PublicHTTPConfig is configured correctly.
func (cfg PublicHTTPConfig) validate() error {
	if !cfg.DeprecatedSG.DeprecatedIngress.IsEmpty() && !cfg.Ingress.IsEmpty() {
		return &errSpecifiedBothIngressFields{
			firstField:  "public.http.security_groups.ingress",
			secondField: "public.http.ingress",
		}
	}
	for idx, certARN := range cfg.Certificates {
		if _, err := arn.Parse(certARN); err != nil {
			return fmt.Errorf(`parse "certificates[%d]": %w`, idx, err)
		}
	}
	if cfg.DeprecatedSG.DeprecatedIngress.VPCIngress != nil {
		return fmt.Errorf("a public load balancer already allows vpc ingress")
	}
	if err := cfg.ELBAccessLogs.validate(); err != nil {
		return fmt.Errorf(`validate "access_logs": %w`, err)
	}
	if err := cfg.DeprecatedSG.validate(); err != nil {
		return err
	}
	return cfg.Ingress.validate()
}

// validate returns nil if ELBAccessLogsArgsOrBool is configured correctly.
func (al ELBAccessLogsArgsOrBool) validate() error {
	if al.isEmpty() {
		return nil
	}
	return al.AdvancedConfig.validate()
}

// validate is a no-op for ELBAccessLogsArgs.
func (al ELBAccessLogsArgs) validate() error {
	return nil
}

// validate returns nil if ALBSecurityGroupsConfig is configured correctly.
func (cfg DeprecatedALBSecurityGroupsConfig) validate() error {
	return cfg.DeprecatedIngress.validate()
}

// validate returns nil if privateHTTPConfig is configured correctly.
func (cfg privateHTTPConfig) validate() error {
	if !cfg.DeprecatedSG.DeprecatedIngress.IsEmpty() && !cfg.Ingress.IsEmpty() {
		return &errSpecifiedBothIngressFields{
			firstField:  "private.http.security_groups.ingress",
			secondField: "private.http.ingress",
		}
	}
	for idx, certARN := range cfg.Certificates {
		if _, err := arn.Parse(certARN); err != nil {
			return fmt.Errorf(`parse "certificates[%d]": %w`, idx, err)
		}
	}
	if !cfg.DeprecatedSG.DeprecatedIngress.RestrictiveIngress.IsEmpty() {
		return fmt.Errorf("an internal load balancer cannot have restrictive ingress fields")
	}
	if err := cfg.DeprecatedSG.validate(); err != nil {
		return fmt.Errorf(`validate "security_groups: %w`, err)
	}
	return cfg.Ingress.validate()
}

// validate returns nil if environmentCDNConfig is configured correctly.
func (cfg EnvironmentCDNConfig) validate() error {
	if cfg.Config.isEmpty() {
		return nil
	}
	return cfg.Config.validate()
}

// validate returns nil if Ingress is configured correctly.
func (i DeprecatedIngress) validate() error {
	if i.IsEmpty() {
		return nil
	}
	return i.RestrictiveIngress.validate()
}

// validate returns nil if RestrictiveIngress is configured correctly.
func (i RestrictiveIngress) validate() error {
	for _, sourceIP := range i.SourceIPs {
		if err := sourceIP.validate(); err != nil {
			return err
		}
	}
	return nil
}

// validate is a no-op for RelaxedIngress.
func (i RelaxedIngress) validate() error {
	return nil
}

// validate returns nil if advancedCDNConfig is configured correctly.
func (cfg AdvancedCDNConfig) validate() error {
	if cfg.Certificate != nil {
		certARN, err := arn.Parse(*cfg.Certificate)
		if err != nil {
			return fmt.Errorf(`parse cdn certificate: %w`, err)
		}
		if certARN.Region != cloudfront.CertRegion {
			return &errInvalidCloudFrontRegion{}
		}
	}
	if err := cfg.Static.validate(); err != nil {
		return fmt.Errorf(`validate "static_assets": %w`, err)
	}
	return nil
}

// validate returns nil if CDNStaticConfig is configured correctly.
func (cfg CDNStaticConfig) validate() error {
	if cfg.IsEmpty() {
		return nil
	}
	if cfg.Alias == "" {
		return &errFieldMustBeSpecified{
			missingField: "alias",
		}
	}
	if cfg.Location == "" {
		return &errFieldMustBeSpecified{
			missingField: "location",
		}
	}
	if cfg.Path == "" {
		return &errFieldMustBeSpecified{
			missingField: "path",
		}
	}
	return nil
}

func (c EnvironmentConfig) validateInternalALBSubnets() error {
	isImported := make(map[string]bool)
	for _, placementSubnet := range c.HTTPConfig.Private.InternalALBSubnets {
		for _, subnet := range append(c.Network.VPC.Subnets.Private, c.Network.VPC.Subnets.Public...) {
			if placementSubnet == aws.StringValue(subnet.SubnetID) {
				isImported[placementSubnet] = true
			}
		}
	}
	if len(isImported) != len(c.HTTPConfig.Private.InternalALBSubnets) {
		return fmt.Errorf("subnet(s) specified for internal ALB placement not imported")
	}
	return nil
}

func areSetsEqual[T comparable](a map[T]struct{}, b map[T]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}
