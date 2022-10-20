// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

// DeprecatedALBSecurityGroupsConfig represents security group configuration settings for an ALB.
type DeprecatedALBSecurityGroupsConfig struct {
	DeprecatedIngress DeprecatedIngress `yaml:"ingress"` // Deprecated. This field is now available inside PublicHTTPConfig.Ingress and privateHTTPConfig.Ingress field.
}

// IsEmpty returns true if there are no specified fields for ingress.
func (cfg DeprecatedALBSecurityGroupsConfig) IsEmpty() bool {
	return cfg.DeprecatedIngress.IsEmpty()
}

// DeprecatedIngress represents allowed ingress traffic from specified fields.
type DeprecatedIngress struct {
	RestrictiveIngress RestrictiveIngress `yaml:"restrict_to"` // Deprecated. This field is no more available in any other field.
	VPCIngress         *bool              `yaml:"from_vpc"`    //Deprecated. This field is now available in privateHTTPConfig.Ingress.VPCIngress
}

// IsEmpty returns true if there are no specified fields for ingress.
func (i DeprecatedIngress) IsEmpty() bool {
	return i.VPCIngress == nil && i.RestrictiveIngress.IsEmpty()
}
