// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/template"
)

type envReadParser interface {
	template.ReadParser
	ParseEnv(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error)
	ParseEnvBootstrap(data *template.EnvOpts, options ...template.ParseOption) (*template.Content, error)
}

// EnvStackConfig is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type EnvStackConfig struct {
	in     *deploy.CreateEnvironmentInput
	parser envReadParser
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

// NewEnvStackConfig sets up a struct that provides values to CloudFormation for deploying an environment.
func NewEnvStackConfig(input *deploy.CreateEnvironmentInput) *EnvStackConfig {
	return &EnvStackConfig{
		in:     input,
		parser: template.New(),
	}
}

// Template returns the environment CloudFormation template.
func (e *EnvStackConfig) Template() (string, error) {
	crs, err := convertCustomResources(e.in.LambdaURLs)
	if err != nil {
		return "", err
	}

	// TODO(Lou1415926): remove all these after we are able to migrate to the new upload workflow.
	bucket, dnsCertValidator, err := s3.ParseURL(e.in.CustomResourcesURLs[template.DNSCertValidatorFileName])
	if err != nil {
		return "", err
	}
	_, dnsDelegation, err := s3.ParseURL(e.in.CustomResourcesURLs[template.DNSDelegationFileName])
	if err != nil {
		return "", err
	}
	_, customDomain, err := s3.ParseURL(e.in.CustomResourcesURLs[template.CustomDomainFileName])
	if err != nil {
		return "", err
	}
	var mft string
	if e.in.Mft != nil {
		out, err := yaml.Marshal(e.in.Mft)
		if err != nil {
			return "", fmt.Errorf("marshal environment manifest to embed in template: %v", err)
		}
		mft = string(out)
	}
	content, err := e.parser.ParseEnv(&template.EnvOpts{
		AppName:                  e.in.App.Name,
		EnvName:                  e.in.Name,
		CustomResources:          crs,
		DNSCertValidatorLambda:   dnsCertValidator,
		DNSDelegationLambda:      dnsDelegation,
		CustomDomainLambda:       customDomain,
		ScriptBucketName:         bucket,
		ArtifactBucketARN:        e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN:     e.in.ArtifactBucketKeyARN,
		PublicImportedCertARNs:   e.importPublicCertARNs(),
		PrivateImportedCertARNs:  e.importPrivateCertARNs(),
		VPCConfig:                e.vpcConfig(),
		CustomInternalALBSubnets: e.internalALBSubnets(),
		AllowVPCIngress:          e.in.AllowVPCIngress, // TODO(jwh): fetch AllowVPCIngress from Manifest or SSM.
		Telemetry:                e.telemetryConfig(),

		Version:       e.in.Version,
		LatestVersion: deploy.LatestEnvTemplateVersion,
		Manifest:      mft,
	}, template.WithFuncs(map[string]interface{}{
		"inc":      template.IncFunc,
		"fmtSlice": template.FmtSliceFunc,
	}))
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
	}, nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized
// to a YAML document annotated with comments for readability to users.
func (e *EnvStackConfig) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
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

func (e *EnvStackConfig) vpcConfig() template.VPCConfig {
	return template.VPCConfig{
		Imported: e.importVPC(),
		Managed:  e.managedVPC(),
	}
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
