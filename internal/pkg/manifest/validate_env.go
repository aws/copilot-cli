// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
)

var (
	errAZsNotEqual = errors.New("public subnets and private subnets do not span the same availability zones")

	minAZs = 2
)

// Validate returns nil if Environment is configured correctly.
func (e Environment) Validate() error {
	if err := e.EnvironmentConfig.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	return nil
}

// Validate returns nil if EnvironmentConfig is configured correctly.
func (e EnvironmentConfig) Validate() error {
	if err := e.Network.Validate(); err != nil {
		return fmt.Errorf(`validate "network": %w`, err)
	}
	if err := e.Observability.Validate(); err != nil {
		return fmt.Errorf(`validate "observability": %w`, err)
	}
	return nil
}

// Validate returns nil if environmentNetworkConfig is configured correctly.
func (n environmentNetworkConfig) Validate() error {
	if err := n.VPC.Validate(); err != nil {
		return fmt.Errorf(`validate "vpc": %w`, err)
	}
	return nil
}

// Validate returns nil if environmentVPCConfig is configured correctly.
func (v environmentVPCConfig) Validate() error {
	if v.imported() && v.managedVPCCustomized() {
		return errors.New(`cannot import VPC resources (with "id" fields) and customize VPC resources (with "cidr" and "az" fields) at the same time`)
	}
	if err := v.Subnets.Validate(); err != nil {
		return fmt.Errorf(`validate "subnets": %w`, err)
	}
	if v.imported() {
		return v.validateImportedVPC()
	}
	if v.managedVPCCustomized() {
		return v.validateManagedVPC()
	}
	return nil
}

func (v environmentVPCConfig) validateImportedVPC() error {
	for idx, subnet := range v.Subnets.Public {
		if aws.StringValue(subnet.SubnetID) == "" {
			return fmt.Errorf(`validate "subnets": public[%d] must include "id" field if the vpc is imported`, idx)
		}
	}
	for idx, subnet := range v.Subnets.Private {
		if aws.StringValue(subnet.SubnetID) == "" {
			return fmt.Errorf(`validate "subnets": private[%d] must include "id" field if the vpc is imported`, idx)
		}
	}
	switch {
	case len(v.Subnets.Private)+len(v.Subnets.Public) <= 0:
		return errors.New(`validate "subnets": VPC must have subnets in order to proceed with environment creation`)
	case len(v.Subnets.Public) == 1:
		return errors.New(`validate "subnets": validate "public": at least two public subnets must be imported to enable Load Balancing`)
	case len(v.Subnets.Private) == 1:
		return errors.New(`validate "subnets": validate "private": at least two private subnets must be imported`)
	}
	return nil
}

func (v environmentVPCConfig) validateManagedVPC() error {
	var (
		publicAZs    = make(map[string]struct{})
		privateAZs   = make(map[string]struct{})
		publicCIDRs  = make(map[string]struct{})
		privateCIDRs = make(map[string]struct{})
	)
	var exists = struct{}{}
	for idx, subnet := range v.Subnets.Public {
		if aws.StringValue((*string)(subnet.CIDR)) == "" || aws.StringValue(subnet.AZ) == "" {
			return fmt.Errorf(`validate "subnets": public[%d] must include "cidr" and "az" fields if the vpc is configured`, idx)
		}
		publicCIDRs[aws.StringValue((*string)(subnet.CIDR))] = exists
		publicAZs[aws.StringValue(subnet.AZ)] = exists
	}
	for idx, subnet := range v.Subnets.Private {
		if aws.StringValue((*string)(subnet.CIDR)) == "" || aws.StringValue(subnet.AZ) == "" {
			return fmt.Errorf(`validate "subnets": private[%d] must include "cidr" and "az" fields if the vpc is configured`, idx)
		}
		privateCIDRs[aws.StringValue((*string)(subnet.CIDR))] = exists
		privateAZs[aws.StringValue(subnet.AZ)] = exists
	}
	if len(publicAZs) != len(privateAZs) {
		return fmt.Errorf(`validate "subnets": %w`, errAZsNotEqual)
	}
	for k := range publicAZs {
		if _, ok := privateAZs[k]; !ok {
			return fmt.Errorf(`validate "subnets": %w`, errAZsNotEqual)
		}
	}
	numAZs := len(publicAZs)
	if numAZs < minAZs {
		return fmt.Errorf(`validate "subnets": require at least %d availability zones`, minAZs)
	}
	if len(publicCIDRs) != numAZs {
		return fmt.Errorf(`validate "subnets": validate "public": number of public subnet CIDRs (%d) does not match number of AZs (%d)`, len(publicCIDRs), len(publicAZs))
	}
	if len(privateCIDRs) != numAZs {
		return fmt.Errorf(`validate "subnets": validate "private": number of private subnet CIDRs (%d) does not match number of AZs (%d)`, len(privateCIDRs), len(publicAZs))
	}
	return nil
}

// Validate returns nil if subnetsConfiguration is configured correctly.
func (cs subnetsConfiguration) Validate() error {
	for idx, subnet := range cs.Public {
		if err := subnet.Validate(); err != nil {
			return fmt.Errorf(`validate "public[%d]": %w`, idx, err)
		}
	}
	for idx, subnet := range cs.Private {
		if err := subnet.Validate(); err != nil {
			return fmt.Errorf(`validate "private[%d]": %w`, idx, err)
		}
	}
	return nil
}

// Validate returns nil if subnetConfiguration is configured correctly.
func (c subnetConfiguration) Validate() error {
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

// Validate returns nil if environmentObservability is configured correctly.
func (o environmentObservability) Validate() error {
	return nil
}

// Validate returns nil if environmentHTTPConfig is configured correctly.
func (o environmentHTTPConfig) Validate() error {
	if err := o.Public.Validate(); err != nil {
		return fmt.Errorf(`validate "public": %w`, err)
	}
	if err := o.Private.Validate(); err != nil {
		return fmt.Errorf(`validate "private": %w`, err)
	}
	return nil
}

// Validate returns nil if publicHTTPConfig is configured correctly.
func (o publicHTTPConfig) Validate() error {
	for idx, certARN := range o.Certificates {
		if _, err := arn.Parse(certARN); err != nil {
			return fmt.Errorf(`validate "certificates[%d]": %w`, idx, err)
		}
	}
	return nil
}

// Validate returns nil if privateHTTPConfig is configured correctly.
func (o privateHTTPConfig) Validate() error {
	for idx, certARN := range o.Certificates {
		if _, err := arn.Parse(certARN); err != nil {
			return fmt.Errorf(`validate "certificates[%d]": %w`, idx, err)
		}
	}
	return nil
}
