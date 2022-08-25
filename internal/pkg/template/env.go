// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"
)

const (
	envCFTemplatePath          = "environment/cf.yml"
	fmtEnvCFSubTemplatePath    = "environment/partials/%s.yml"
	envBootstrapCFTemplatePath = "environment/bootstrap-cf.yml"
)

// Available env-controller managed feature names.
const (
	ALBFeatureName         = "ALBWorkloads"
	EFSFeatureName         = "EFSWorkloads"
	NATFeatureName         = "NATWorkloads"
	InternalALBFeatureName = "InternalALBWorkloads"
	AliasesFeatureName     = "Aliases"
)

// LastForceDeployIDOutputName is the logical ID of the deployment controller output.
const LastForceDeployIDOutputName = "LastForceDeployID"

var friendlyEnvFeatureName = map[string]string{
	ALBFeatureName:         "ALB",
	EFSFeatureName:         "EFS",
	NATFeatureName:         "NAT Gateway",
	InternalALBFeatureName: "Internal ALB",
	AliasesFeatureName:     "Aliases",
}

var leastVersionForFeature = map[string]string{
	ALBFeatureName:         "v1.0.0",
	EFSFeatureName:         "v1.3.0",
	NATFeatureName:         "v1.3.0",
	InternalALBFeatureName: "v1.10.0",
	AliasesFeatureName:     "v1.4.0",
}

// AvailableEnvFeatures returns a list of the latest available feature, named after their corresponding parameter names.
func AvailableEnvFeatures() []string {
	return []string{ALBFeatureName, EFSFeatureName, NATFeatureName, InternalALBFeatureName, AliasesFeatureName}
}

// FriendlyEnvFeatureName returns a user-friendly feature name given a env-controller managed parameter name.
// If there isn't one, it returns the parameter name that it is given.
func FriendlyEnvFeatureName(feature string) string {
	friendly, ok := friendlyEnvFeatureName[feature]
	if !ok {
		return feature
	}
	return friendly
}

// LeastVersionForFeature maps each feature to the least environment template version it requires.
func LeastVersionForFeature(feature string) string {
	return leastVersionForFeature[feature]
}

var (
	// Template names under "environment/partials/".
	envCFSubTemplateNames = []string{
		"cdn-resources",
		"cfn-execution-role",
		"custom-resources",
		"custom-resources-role",
		"environment-manager-role",
		"lambdas",
		"vpc-resources",
		"nat-gateways",
		"bootstrap-resources",
		"elb-access-logs",
		"mappings-regional-configs",
	}
)

var (
	// Template names under "environment/partials/".
	bootstrapEnvSubTemplateName = []string{
		"cfn-execution-role",
		"environment-manager-role",
		"bootstrap-resources",
	}
)

// EnvOpts holds data that can be provided to enable features in an environment stack template.
type EnvOpts struct {
	AppName string // The application name. Needed to create default value for svc discovery endpoint for upgraded environments.
	EnvName string
	Version string // The template version to use for the environment. If empty uses the "legacy" template.

	// Custom Resourced backed by Lambda functions.
	CustomResources           map[string]S3ObjectLocation
	DNSDelegationLambda       string
	DNSCertValidatorLambda    string
	EnableLongARNFormatLambda string
	CustomDomainLambda        string

	ScriptBucketName     string
	ArtifactBucketARN    string
	ArtifactBucketKeyARN string

	VPCConfig         VPCConfig
	PublicHTTPConfig  HTTPConfig
	PrivateHTTPConfig HTTPConfig
	Telemetry         *Telemetry
	CDNConfig         *CDNConfig

	LatestVersion      string
	SerializedManifest string // Serialized manifest used to render the environment template.
	ForceUpdateID      string

	DelegateDNS bool
}

// HTTPConfig represents configuration for a Load Balancer.
type HTTPConfig struct {
	CIDRPrefixListIDs []string
	ImportedCertARNs  []string
	CustomALBSubnets  []string
	ELBAccessLogs     *ELBAccessLogs
}

// ELBAccessLogs represents configuration for ELB access logs S3 bucket.
type ELBAccessLogs struct {
	BucketName string
	Prefix     string
}

// ShouldCreateBucket returns true if copilot should create bucket on behalf of customer.
func (elb *ELBAccessLogs) ShouldCreateBucket() bool {
	if elb == nil {
		return false
	}
	return elb.BucketName == ""
}

// CDNConfig represents a Content Delivery Network deployed by CloudFront.
type CDNConfig struct {
	ImportedCertificate *string
}

type VPCConfig struct {
	Imported            *ImportVPC // If not-nil, use the imported VPC resources instead of the Managed VPC.
	Managed             ManagedVPC
	AllowVPCIngress     bool
	SecurityGroupConfig *SecurityGroupConfig
}

// ImportVPC holds the fields to import VPC resources.
type ImportVPC struct {
	ID               string
	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
}

// ManagedVPC holds the fields to configure a managed VPC.
type ManagedVPC struct {
	CIDR               string
	AZs                []string
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

// Telemetry represents optional observability and monitoring configuration.
type Telemetry struct {
	EnableContainerInsights bool
}

// SecurityGroupConfig holds the fields to import security group config
type SecurityGroupConfig struct {
	Ingress []SecurityGroupRule
	Egress  []SecurityGroupRule
}

// SecurityGroupRule holds the fields to import security group rule
type SecurityGroupRule struct {
	CidrIP     string
	FromPort   int
	IpProtocol string
	ToPort     int
}

// ParseEnv parses an environment's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseEnv(data *EnvOpts) (*Content, error) {
	tpl, err := t.parse("base", envCFTemplatePath, withEnvParsingFuncs())
	if err != nil {
		return nil, err
	}
	for _, templateName := range envCFSubTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtEnvCFSubTemplatePath, templateName), withEnvParsingFuncs())
		if err != nil {
			return nil, err
		}
		_, err = tpl.AddParseTree(templateName, nestedTpl.Tree)
		if err != nil {
			return nil, fmt.Errorf("add parse tree of %s to base template: %w", templateName, err)
		}
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute environment template with data %v: %w", data, err)
	}
	return &Content{buf}, nil
}

// ParseEnvBootstrap parses the CloudFormation template that bootstrap IAM resources with the specified data object and returns its content.
func (t *Template) ParseEnvBootstrap(data *EnvOpts, options ...ParseOption) (*Content, error) {
	tpl, err := t.parse("base", envBootstrapCFTemplatePath, options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range bootstrapEnvSubTemplateName {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtEnvCFSubTemplatePath, templateName), options...)
		if err != nil {
			return nil, err
		}
		_, err = tpl.AddParseTree(templateName, nestedTpl.Tree)
		if err != nil {
			return nil, fmt.Errorf("add parse tree of %s to base template: %w", templateName, err)
		}
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute environment template with data %v: %w", data, err)
	}
	return &Content{buf}, nil
}

func withEnvParsingFuncs() ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(map[string]interface{}{
			"inc":      IncFunc,
			"fmtSlice": FmtSliceFunc,
			"quote":    strconv.Quote,
		})
	}
}
