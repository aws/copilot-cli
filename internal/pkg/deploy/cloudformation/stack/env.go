// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/google/uuid"
)

type envReadParser interface {
	template.ReadParser
	ParseEnv(data *template.EnvOpts) (*template.Content, error)
	ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error)
}

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	in                *deploy.CreateEnvironmentInput
	lastForceUpdateID string
	prevParams        []*cloudformation.Parameter
	parser            envReadParser
}

const (
	// Parameter keys.
	envParamAppNameKey                     = "AppName"
	envParamEnvNameKey                     = "EnvironmentName"
	envParamToolsAccountPrincipalKey       = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                      = "AppDNSName"
	envParamAppDNSDelegationRoleKey        = "AppDNSDelegationRole"
	EnvParamAliasesKey                     = "Aliases"
	EnvParamALBWorkloadsKey                = "ALBWorkloads"
	envParamInternalALBWorkloadsKey        = "InternalALBWorkloads"
	envParamEFSWorkloadsKey                = "EFSWorkloads"
	envParamNATWorkloadsKey                = "NATWorkloads"
	envParamCreateHTTPSListenerKey         = "CreateHTTPSListener"
	envParamCreateInternalHTTPSListenerKey = "CreateInternalHTTPSListener"
	EnvParamServiceDiscoveryEndpoint       = "ServiceDiscoveryEndpoint"

	// Output keys.
	EnvOutputVPCID               = "VpcId"
	EnvOutputPublicSubnets       = "PublicSubnets"
	EnvOutputPrivateSubnets      = "PrivateSubnets"
	envOutputCFNExecutionRoleARN = "CFNExecutionRoleARN"
	envOutputManagerRoleKey      = "EnvironmentManagerRoleARN"

	// Default parameter values.
	DefaultVPCCIDR = "10.0.0.0/16"
)

var (
	fmtServiceDiscoveryEndpoint = "%s.%s.local"
	DefaultPublicSubnetCIDRs    = []string{"10.0.0.0/24", "10.0.1.0/24"}
	DefaultPrivateSubnetCIDRs   = []string{"10.0.2.0/24", "10.0.3.0/24"}
)

// NewEnvStackConfig return o CloudFormation stack configuration for deploying a brand new environment.
func NewEnvStackConfig(input *deploy.CreateEnvironmentInput) *EnvStackConfig {
	return &EnvStackConfig{
		in:     input,
		parser: template.New(),
	}
}

// NewEnvConfigFromExistingStack returns a CloudFormation stack configuration for updating an environment.
func NewEnvConfigFromExistingStack(in *deploy.CreateEnvironmentInput, lastForceUpdateID string, prevParams []*cloudformation.Parameter) *EnvStackConfig {
	return &EnvStackConfig{
		in:                in,
		prevParams:        prevParams,
		lastForceUpdateID: lastForceUpdateID,
		parser:            template.New(),
	}
}

// Template returns the environment CloudFormation template.
func (e *EnvStackConfig) Template() (string, error) {
	crs, err := convertCustomResources(e.in.CustomResourcesURLs)
	if err != nil {
		return "", err
	}
	forceUpdateID := e.lastForceUpdateID
	if e.in.ForceUpdate {
		id, err := uuid.NewRandom()
		if err != nil {
			return "", fmt.Errorf("generate uuid for a forced update: %s", err)
		}
		forceUpdateID = id.String()
	}

	publicHTTPConfig, err := e.publicHTTPConfig()
	if err != nil {
		return "", err
	}

	vpcConfig, err := e.vpcConfig()
	if err != nil {
		return "", err
	}

	content, err := e.parser.ParseEnv(&template.EnvOpts{
		AppName:              e.in.App.Name,
		EnvName:              e.in.Name,
		CustomResources:      crs,
		ArtifactBucketARN:    e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN: e.in.ArtifactBucketKeyARN,
		PublicHTTPConfig:     publicHTTPConfig,
		VPCConfig:            vpcConfig,
		PrivateHTTPConfig:    e.privateHTTPConfig(),
		Telemetry:            e.telemetryConfig(),
		CDNConfig:            e.cdnConfig(),

		Version:            e.in.Version,
		LatestVersion:      deploy.LatestEnvTemplateVersion,
		SerializedManifest: string(e.in.RawMft),
		ForceUpdateID:      forceUpdateID,
		DelegateDNS:        e.in.App.Domain != "",
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the parameters to be passed into an environment CloudFormation template.
func (e *EnvStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	httpsListener := "false"
	if len(e.importPublicCertARNs()) != 0 || e.in.App.Domain != "" {
		httpsListener = "true"
	}
	internalHTTPSListener := "false"
	if len(e.importPrivateCertARNs()) != 0 {
		internalHTTPSListener = "true"
	}
	currParams := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamAppNameKey),
			ParameterValue: aws.String(e.in.App.Name),
		},
		{
			ParameterKey:   aws.String(envParamEnvNameKey),
			ParameterValue: aws.String(e.in.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
			ParameterValue: aws.String(e.in.App.AccountPrincipalARN),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSKey),
			ParameterValue: aws.String(e.in.App.Domain),
		},
		{
			ParameterKey:   aws.String(envParamAppDNSDelegationRoleKey),
			ParameterValue: aws.String(e.in.App.DNSDelegationRole()),
		},
		{
			ParameterKey:   aws.String(EnvParamServiceDiscoveryEndpoint),
			ParameterValue: aws.String(fmt.Sprintf(fmtServiceDiscoveryEndpoint, e.in.Name, e.in.App.Name)),
		},
		{
			ParameterKey:   aws.String(envParamCreateHTTPSListenerKey),
			ParameterValue: aws.String(httpsListener),
		},
		{
			ParameterKey:   aws.String(envParamCreateInternalHTTPSListenerKey),
			ParameterValue: aws.String(internalHTTPSListener),
		},
		{
			ParameterKey:   aws.String(EnvParamAliasesKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(EnvParamALBWorkloadsKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(envParamInternalALBWorkloadsKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(envParamEFSWorkloadsKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(envParamNATWorkloadsKey),
			ParameterValue: aws.String(""),
		},
	}
	if e.prevParams == nil {
		return currParams, nil
	}
	// If we're creating a stack configuration for an existing environment stack, ensure the previous env controller
	// managed parameters are using the previous value.
	return e.transformParameters(currParams, e.prevParams, transformEnvControllerParameters)
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (e *EnvStackConfig) SerializedParameters() (string, error) {
	return serializeTemplateConfig(e.parser, e)
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *EnvStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(e.in.AdditionalTags, map[string]string{
		deploy.AppTagKey: e.in.App.Name,
		deploy.EnvTagKey: e.in.Name,
	})
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *EnvStackConfig) StackName() string {
	return NameForEnv(e.in.App.Name, e.in.Name)
}

// transformParameters removes or transforms each of the current parameters and does not add any new parameters.
// This means that parameters that exist only in the old template are left out.
// The parameter`transform` is a function that transform a parameter, given its value in the new template and the old template.
// If `old` is `nil`, the parameter does not exist in the old template.
// `transform` should return `nil` if caller intends to delete the parameter.
func (e *EnvStackConfig) transformParameters(
	currParams []*cloudformation.Parameter,
	oldParams []*cloudformation.Parameter,
	transform func(new cloudformation.Parameter, old *cloudformation.Parameter) *cloudformation.Parameter) ([]*cloudformation.Parameter, error) {

	// Make a map out of `currParams` and out of `oldParams`.
	curr := make(map[string]cloudformation.Parameter)
	for _, p := range currParams {
		curr[aws.StringValue(p.ParameterKey)] = *p
	}
	old := make(map[string]*cloudformation.Parameter)
	for _, p := range oldParams {
		old[aws.StringValue(p.ParameterKey)] = p
	}

	// Remove or transform each of the current parameters.
	var params []*cloudformation.Parameter
	for k, p := range curr {
		if transformed := transform(p, old[k]); transformed != nil {
			params = append(params, transformed)
		}
	}
	return params, nil
}

// transformEnvControllerParameters transforms a parameter such that it uses its previous value if:
// 1. The parameter exists in the old template.
// 2. The parameter is env-controller managed.
// Otherwise, it returns the parameter untouched.
func transformEnvControllerParameters(new cloudformation.Parameter, old *cloudformation.Parameter) *cloudformation.Parameter {
	if old == nil { // The ParamKey doesn't exist in the old stack, use the new value.
		return &new
	}

	var (
		isEnvControllerManaged = make(map[string]struct{})
		exists                 = struct{}{}
	)
	for _, f := range template.AvailableEnvFeatures() {
		isEnvControllerManaged[f] = exists
	}
	if _, ok := isEnvControllerManaged[aws.StringValue(new.ParameterKey)]; !ok {
		return &new
	}
	return &cloudformation.Parameter{
		ParameterKey: new.ParameterKey,

		// Ideally, we would set `UsePreviousValue: true` unfortunately CodePipeline template config does not support it.
		// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/continuous-delivery-codepipeline-cfn-artifacts.html#w2ab1c21c15c15
		ParameterValue: old.ParameterValue,
	}
}

// NewBootstrapEnvStackConfig sets up a BootstrapEnvStackConfig struct.
func NewBootstrapEnvStackConfig(input *deploy.CreateEnvironmentInput) *BootstrapEnvStackConfig {
	return &BootstrapEnvStackConfig{
		in:     input,
		parser: template.New(),
	}
}

// BootstrapEnvStackConfig contains information for creating a stack bootstrapping environment resources.
type BootstrapEnvStackConfig EnvStackConfig

// Template returns the CloudFormation template to bootstrap environment resources.
func (e *BootstrapEnvStackConfig) Template() (string, error) {
	content, err := e.parser.ParseEnvBootstrap(&template.EnvOpts{
		ArtifactBucketARN:    e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN: e.in.ArtifactBucketKeyARN,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the parameters to be passed into the bootstrap stack's CloudFormation template.
func (e *BootstrapEnvStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(envParamAppNameKey),
			ParameterValue: aws.String(e.in.App.Name),
		},
		{
			ParameterKey:   aws.String(envParamEnvNameKey),
			ParameterValue: aws.String(e.in.Name),
		},
		{
			ParameterKey:   aws.String(envParamToolsAccountPrincipalKey),
			ParameterValue: aws.String(e.in.App.AccountPrincipalARN),
		},
	}, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (e *BootstrapEnvStackConfig) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
}

// Tags returns the tags that should be applied to the bootstrap CloudFormation stack.
func (e *BootstrapEnvStackConfig) Tags() []*cloudformation.Tag {
	return (*EnvStackConfig)(e).Tags()
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *BootstrapEnvStackConfig) StackName() string {
	return (*EnvStackConfig)(e).StackName()
}

// ToEnv inspects an environment cloudformation stack and constructs an environment
// struct out of it (including resources like ECR Repo).
func (e *BootstrapEnvStackConfig) ToEnv(stack *cloudformation.Stack) (*config.Environment, error) {
	stackARN, err := arn.Parse(*stack.StackId)
	if err != nil {
		return nil, fmt.Errorf("couldn't extract region and account from stack ID %s: %w", *stack.StackId, err)
	}

	stackOutputs := make(map[string]string)
	for _, output := range stack.Outputs {
		stackOutputs[*output.OutputKey] = *output.OutputValue
	}

	return &config.Environment{
		Name:             e.in.Name,
		App:              e.in.App.Name,
		Region:           stackARN.Region,
		AccountID:        stackARN.AccountID,
		ManagerRoleARN:   stackOutputs[envOutputManagerRoleKey],
		ExecutionRoleARN: stackOutputs[envOutputCFNExecutionRoleARN],
	}, nil
}

func (e *EnvStackConfig) cdnConfig() *template.CDNConfig {
	if e.in.Mft == nil {
		return nil
	}
	if !e.in.Mft.CDNEnabled() {
		return nil
	}
	return &template.CDNConfig{
		ImportedCertificate: e.in.Mft.CDNConfig.Config.Certificate,
	}
}

func (e *EnvStackConfig) publicHTTPConfig() (template.HTTPConfig, error) {
	elbAccessLogsConfig, err := convertELBAccessLogsConfig(e.in.Mft)
	if err != nil {
		return template.HTTPConfig{}, err
	}
	return template.HTTPConfig{
		CIDRPrefixListIDs: e.in.CIDRPrefixListIDs,
		ImportedCertARNs:  e.importPublicCertARNs(),
		ELBAccessLogs:     elbAccessLogsConfig,
	}, nil
}

func (e *EnvStackConfig) privateHTTPConfig() template.HTTPConfig {
	return template.HTTPConfig{
		ImportedCertARNs: e.importPrivateCertARNs(),
		CustomALBSubnets: e.internalALBSubnets(),
	}
}

func (e *EnvStackConfig) vpcConfig() (template.VPCConfig, error) {
	securityGroupConfig, err := convertEnvSecurityGroupCfg(e.in.Mft)
	if err != nil {
		return template.VPCConfig{}, err
	}
	return template.VPCConfig{
		Imported:            e.importVPC(),
		Managed:             e.managedVPC(),
		AllowVPCIngress:     aws.BoolValue(e.in.Mft.HTTPConfig.Private.SecurityGroupsConfig.Ingress.VPCIngress),
		SecurityGroupConfig: securityGroupConfig,
	}, nil
}

func (e *EnvStackConfig) importVPC() *template.ImportVPC {
	// If a manifest is present, it is the only place we look at.
	if e.in.Mft != nil {
		return e.in.Mft.Network.VPC.ImportedVPC()
	}

	// Fallthrough to SSM config.
	if e.in.ImportVPCConfig == nil {
		return nil
	}
	return &template.ImportVPC{
		ID:               e.in.ImportVPCConfig.ID,
		PublicSubnetIDs:  e.in.ImportVPCConfig.PublicSubnetIDs,
		PrivateSubnetIDs: e.in.ImportVPCConfig.PrivateSubnetIDs,
	}
}

func (e *EnvStackConfig) managedVPC() template.ManagedVPC {
	defaultManagedVPC := template.ManagedVPC{
		CIDR:               DefaultVPCCIDR,
		PublicSubnetCIDRs:  DefaultPublicSubnetCIDRs,
		PrivateSubnetCIDRs: DefaultPrivateSubnetCIDRs,
	}
	// If a manifest is present, it is the only place we look at.
	if e.in.Mft != nil {
		if v := e.in.Mft.Network.VPC.ManagedVPC(); v != nil {
			return *v
		}
		return defaultManagedVPC
	}

	// Fallthrough to SSM config.
	if e.in.AdjustVPCConfig == nil {
		return defaultManagedVPC
	}
	return template.ManagedVPC{
		CIDR:               e.in.AdjustVPCConfig.CIDR,
		AZs:                e.in.AdjustVPCConfig.AZs,
		PublicSubnetCIDRs:  e.in.AdjustVPCConfig.PublicSubnetCIDRs,
		PrivateSubnetCIDRs: e.in.AdjustVPCConfig.PrivateSubnetCIDRs,
	}
}

func (e *EnvStackConfig) telemetryConfig() *template.Telemetry {
	// If a manifest is present, it is the only place we look at.
	if e.in.Mft != nil {
		return &template.Telemetry{
			EnableContainerInsights: aws.BoolValue(e.in.Mft.Observability.ContainerInsights),
		}
	}

	// Fallthrough to SSM config.
	if e.in.Telemetry == nil {
		// For environments before Copilot v1.14.0, `Telemetry` is nil.
		return nil
	}
	return &template.Telemetry{
		// For environments after v1.14.0, and v1.20.0, `Telemetry` is never nil,
		// and `EnableContainerInsights` is either true or false.
		EnableContainerInsights: e.in.Telemetry.EnableContainerInsights,
	}
}

func (e *EnvStackConfig) importPublicCertARNs() []string {
	// If a manifest is present, it is the only place we look at.
	if e.in.Mft != nil {
		return e.in.Mft.HTTPConfig.Public.Certificates
	}
	// Fallthrough to SSM config.
	if e.in.ImportVPCConfig != nil && len(e.in.ImportVPCConfig.PublicSubnetIDs) == 0 {
		return nil
	}
	return e.in.ImportCertARNs
}

func (e *EnvStackConfig) importPrivateCertARNs() []string {
	// If a manifest is present, it is the only place we look at.
	if e.in.Mft != nil {
		return e.in.Mft.HTTPConfig.Private.Certificates
	}
	// Fallthrough to SSM config.
	if e.in.ImportVPCConfig != nil && len(e.in.ImportVPCConfig.PublicSubnetIDs) == 0 {
		return e.in.ImportCertARNs
	}
	return nil
}

func (e *EnvStackConfig) internalALBSubnets() []string {
	// If a manifest is present, it is the only place we look.
	if e.in.Mft != nil {
		return e.in.Mft.HTTPConfig.Private.InternalALBSubnets
	}
	// Fallthrough to SSM config.
	return e.in.InternalALBSubnets
}
