// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"

	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/google/uuid"
)

// Parameter keys.
const (
	EnvParamAliasesKey                     = "Aliases"
	EnvParamALBWorkloadsKey                = "ALBWorkloads"
	EnvParamServiceDiscoveryEndpoint       = "ServiceDiscoveryEndpoint"
	envParamAppNameKey                     = "AppName"
	envParamEnvNameKey                     = "EnvironmentName"
	envParamToolsAccountPrincipalKey       = "ToolsAccountPrincipalARN"
	envParamAppDNSKey                      = "AppDNSName"
	envParamAppDNSDelegationRoleKey        = "AppDNSDelegationRole"
	envParamInternalALBWorkloadsKey        = "InternalALBWorkloads"
	envParamEFSWorkloadsKey                = "EFSWorkloads"
	envParamNATWorkloadsKey                = "NATWorkloads"
	envParamAppRunnerPrivateWorkloadsKey   = "AppRunnerPrivateWorkloads"
	envParamCreateHTTPSListenerKey         = "CreateHTTPSListener"
	envParamCreateInternalHTTPSListenerKey = "CreateInternalHTTPSListener"
)

// Output keys.
const (
	EnvOutputVPCID               = "VpcId"
	EnvOutputPublicSubnets       = "PublicSubnets"
	EnvOutputPrivateSubnets      = "PrivateSubnets"
	envOutputCFNExecutionRoleARN = "CFNExecutionRoleARN"
	envOutputManagerRoleKey      = "EnvironmentManagerRoleARN"
)

// Cloudformation stack tag keys.
const (
	StackNameTagKey = "aws:cloudformation:stack-name"
	LogicalIDTagKey = "aws:cloudformation:logical-id"
)

// Environment managed S3 buckets.
const (
	ELBAccessLogsBucketLogicalID = "ELBAccessLogsBucket"
)

// Slice of environment managed S3 bucket IDs.
var (
	EnvManagedS3BucketLogicalIds = []string{
		ELBAccessLogsBucketLogicalID,
	}
)

const (
	// DefaultVPCCIDR is the default CIDR used for a manged VPC.
	DefaultVPCCIDR = "10.0.0.0/16"
)

var (
	// DefaultPublicSubnetCIDRs contains two default CIDRs for the two managed public subnets.
	DefaultPublicSubnetCIDRs = []string{"10.0.0.0/24", "10.0.1.0/24"}
	// DefaultPrivateSubnetCIDRs contains two default CIDRs for the two managed private subnets.
	DefaultPrivateSubnetCIDRs = []string{"10.0.2.0/24", "10.0.3.0/24"}
)

var fmtServiceDiscoveryEndpoint = "%s.%s.local"

// Addons contains information about a packaged addons.
type Addons struct {
	S3ObjectURL string                // S3ObjectURL is the URL where the addons template is uploaded.
	Stack       NestedStackConfigurer // Stack generates the template and the parameters to the addons.
}

// EnvConfig holds the fields required to deploy an environment.
type EnvConfig struct {
	Name    string // Name of the environment, must be unique within an application.
	Version string // The version of the environment template to create the stack. If empty, creates the legacy stack.

	// Application regional configurations.
	App                  deploy.AppInformation // Information about the application that the environment belongs to, include app name, DNS name, the principal ARN of the account.
	AdditionalTags       map[string]string     // AdditionalTags are labels applied to resources under the application.
	ArtifactBucketARN    string                // ARN of the regional application bucket.
	ArtifactBucketKeyARN string                // ARN of the KMS key used to encrypt the contents in the regional application bucket.
	PermissionsBoundary  string                // Optional. An IAM Managed Policy name used as permissions boundary for IAM roles.

	// Runtime configurations.
	Addons              *Addons
	CustomResourcesURLs map[string]string //  Mapping of Custom Resource Function Name to the S3 URL where the function zip file is stored.

	// User inputs.
	ImportVPCConfig     *config.ImportVPC     // Optional configuration if users have an existing VPC.
	AdjustVPCConfig     *config.AdjustVPC     // Optional configuration if users want to override default VPC configuration.
	ImportCertARNs      []string              // Optional configuration if users want to import certificates.
	InternalALBSubnets  []string              // Optional configuration if users want to specify internal ALB placement.
	AllowVPCIngress     bool                  // Optional configuration to allow access to internal ALB from ports 80/443.
	CIDRPrefixListIDs   []string              // Optional configuration to specify public security group ingress based on prefix lists.
	PublicALBSourceIPs  []string              // Optional configuration to specify public security group ingress based on customer given source IPs.
	InternalLBSourceIPs []string              // Optional configuration to specify private security group ingress based on customer given source IPs.
	Telemetry           *config.Telemetry     // Optional observability and monitoring configuration.
	Mft                 *manifest.Environment // Unmarshaled and interpolated manifest object.
	RawMft              string                // Content of the environment manifest with env var interpolation only.
	ForceUpdate         bool
}

func (cfg *EnvConfig) loadCustomResourceURLs(crs []uploadable) error {
	if len(cfg.CustomResourcesURLs) != 0 {
		return nil
	}
	bucket, _, err := s3.ParseARN(cfg.ArtifactBucketARN)
	if err != nil {
		return fmt.Errorf("parse artifact bucket ARN: %w", err)
	}
	cfg.CustomResourcesURLs = make(map[string]string, len(crs))
	for _, cr := range crs {
		cfg.CustomResourcesURLs[cr.Name()] = s3.Location(bucket, cr.ArtifactPath())
	}
	return nil
}

// Env is for providing all the values to set up an
// environment stack and to interpret the outputs from it.
type Env struct {
	in                *EnvConfig
	lastForceUpdateID string
	prevParams        []*cloudformation.Parameter
	parser            envReadParser
}

// NewEnvStackConfig returns a CloudFormation stack configuration for deploying a brand-new environment.
func NewEnvStackConfig(input *EnvConfig) (*Env, error) {
	crs, err := customresource.Env(fs)
	if err != nil {
		return nil, fmt.Errorf("environment custom resources: %w", err)
	}
	if err := input.loadCustomResourceURLs(uploadableCRs(crs).convert()); err != nil {
		return nil, err
	}
	return &Env{
		in:     input,
		parser: fs,
	}, nil
}

// NewEnvConfigFromExistingStack returns a CloudFormation stack configuration for updating an environment.
func NewEnvConfigFromExistingStack(in *EnvConfig, lastForceUpdateID string, prevParams []*cloudformation.Parameter) (*Env, error) {
	crs, err := customresource.Env(fs)
	if err != nil {
		return nil, fmt.Errorf("environment custom resources: %w", err)
	}
	if err := in.loadCustomResourceURLs(uploadableCRs(crs).convert()); err != nil {
		return nil, err
	}
	return &Env{
		in:                in,
		prevParams:        prevParams,
		lastForceUpdateID: lastForceUpdateID,
		parser:            fs,
	}, nil
}

// Template returns the environment CloudFormation template.
func (e *Env) Template() (string, error) {
	crs, err := convertCustomResources(e.in.CustomResourcesURLs)
	if err != nil {
		return "", err
	}
	var addons *template.Addons
	if e.in.Addons != nil {
		extraParams, err := e.in.Addons.Stack.Parameters()
		if err != nil {
			return "", fmt.Errorf("parse extra parameters for environment addons: %w", err)
		}
		addons = &template.Addons{
			URL:         e.in.Addons.S3ObjectURL,
			ExtraParams: extraParams,
		}
	}
	vpcConfig, err := e.vpcConfig()
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
	content, err := e.parser.ParseEnv(&template.EnvOpts{
		AppName:              e.in.App.Name,
		EnvName:              e.in.Name,
		CustomResources:      crs,
		Addons:               addons,
		ArtifactBucketARN:    e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN: e.in.ArtifactBucketKeyARN,
		PermissionsBoundary:  e.in.PermissionsBoundary,
		PublicHTTPConfig:     e.publicHTTPConfig(),
		VPCConfig:            vpcConfig,
		PrivateHTTPConfig:    e.privateHTTPConfig(),
		Telemetry:            e.telemetryConfig(),
		CDNConfig:            e.cdnConfig(),

		LatestVersion:      e.in.Version,
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
func (e *Env) Parameters() ([]*cloudformation.Parameter, error) {
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
		{
			ParameterKey:   aws.String(envParamAppRunnerPrivateWorkloadsKey),
			ParameterValue: aws.String(""),
		},
	}
	if e.prevParams == nil {
		return currParams, nil
	}
	// If we're creating a stack configuration for an existing environment stack, ensure the previous env controller
	// managed parameters are using the previous value.
	return e.transformParameters(currParams, e.prevParams, transformEnvControllerParameters, e.transformServiceDiscoveryEndpoint)
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (e *Env) SerializedParameters() (string, error) {
	return serializeTemplateConfig(e.parser, e)
}

// Tags returns the tags that should be applied to the environment CloudFormation stack.
func (e *Env) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(e.in.AdditionalTags, map[string]string{
		deploy.AppTagKey: e.in.App.Name,
		deploy.EnvTagKey: e.in.Name,
	})
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *Env) StackName() string {
	return NameForEnv(e.in.App.Name, e.in.Name)
}

type transformParameterFunc func(new, old *cloudformation.Parameter) *cloudformation.Parameter

// transformParameters removes or transforms each of the current parameters and does not add any new parameters.
// This means that parameters that exist only in the old template are left out.
// The parameter`transformFunc` are functions that transform a parameter, given its value in the new template and the old template.
// Each transform functions should keep the following in mind:
// 1. It should return `nil` if the parameter should be removed.
// 2. The transform functions are applied in a convolutional manner.
// 3. If the parameter `old` is passed in as `nil`, the parameter does not exist in the old template.
func (e *Env) transformParameters(currParams, oldParams []*cloudformation.Parameter, transformFunc ...transformParameterFunc) ([]*cloudformation.Parameter, error) {
	// Make a map out of `currParams` and out of `oldParams`.
	curr := make(map[string]*cloudformation.Parameter)
	for _, p := range currParams {
		curr[aws.StringValue(p.ParameterKey)] = p
	}
	old := make(map[string]*cloudformation.Parameter)
	for _, p := range oldParams {
		old[aws.StringValue(p.ParameterKey)] = p
	}

	// Remove or transform each of the current parameters.
	var params []*cloudformation.Parameter
	for k, p := range curr {
		currP := p
		for _, transform := range transformFunc {
			currP = transform(currP, old[k])
		}
		if currP != nil {
			params = append(params, currP)
		}
	}
	return params, nil
}

// transformEnvControllerParameters transforms an env-controller managed parameter.
// If the parameter exists in the old template, it returns the old parameter assuming that old.ParameterKey = new.ParameterKey.
// Otherwise, it returns its new default value.
func transformEnvControllerParameters(new, old *cloudformation.Parameter) *cloudformation.Parameter {
	if new == nil {
		return nil
	}
	isEnvControllerManaged := make(map[string]struct{})
	for _, f := range template.AvailableEnvFeatures() {
		isEnvControllerManaged[f] = struct{}{}
	}
	if _, ok := isEnvControllerManaged[aws.StringValue(new.ParameterKey)]; !ok {
		return new
	}
	if old == nil { // The EnvController-managed parameter doesn't exist in the old stack. Use the new value.
		return new
	}
	// Ideally, we would return `&cloudformation.Parameter{ ParameterKey: new.ParameterKey, UsePreviousValue: true}`.
	// Unfortunately CodePipeline template config does not support it.
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/continuous-delivery-codepipeline-cfn-artifacts.html#w2ab1c21c15c15
	return old
}

// transformServiceDiscoveryEndpoint transforms the service discovery endpoint parameter.
// If the parameter exists in the old template, it uses its previous value.
// Otherwise, it uses a default value of `<app>.local`.
func (e *Env) transformServiceDiscoveryEndpoint(new, old *cloudformation.Parameter) *cloudformation.Parameter {
	if new == nil {
		return nil
	}
	if aws.StringValue(new.ParameterKey) != EnvParamServiceDiscoveryEndpoint {
		return new
	}
	if old == nil {
		return &cloudformation.Parameter{
			ParameterKey:   new.ParameterKey,
			ParameterValue: aws.String(fmt.Sprintf(`%s.local`, e.in.App.Name)),
		}
	}
	// Ideally, we would return `&cloudformation.Parameter{ ParameterKey: new.ParameterKey, UsePreviousValue: true}`.
	// Unfortunately CodePipeline template config does not support it.
	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/continuous-delivery-codepipeline-cfn-artifacts.html#w2ab1c21c15c15
	return old
}

// NewBootstrapEnvStackConfig sets up a BootstrapEnv struct.
func NewBootstrapEnvStackConfig(input *EnvConfig) *BootstrapEnv {
	return &BootstrapEnv{
		in:     input,
		parser: template.New(),
	}
}

// BootstrapEnv contains information for creating a stack bootstrapping environment resources.
type BootstrapEnv Env

// Template returns the CloudFormation template to bootstrap environment resources.
func (e *BootstrapEnv) Template() (string, error) {
	content, err := e.parser.ParseEnvBootstrap(&template.EnvOpts{
		ArtifactBucketARN:    e.in.ArtifactBucketARN,
		ArtifactBucketKeyARN: e.in.ArtifactBucketKeyARN,
		PermissionsBoundary:  e.in.PermissionsBoundary,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Parameters returns the parameters to be passed into the bootstrap stack's CloudFormation template.
func (e *BootstrapEnv) Parameters() ([]*cloudformation.Parameter, error) {
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
func (e *BootstrapEnv) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
}

// Tags returns the tags that should be applied to the bootstrap CloudFormation stack.
func (e *BootstrapEnv) Tags() []*cloudformation.Tag {
	return (*Env)(e).Tags()
}

// StackName returns the name of the CloudFormation stack (based on the app and env names).
func (e *BootstrapEnv) StackName() string {
	return (*Env)(e).StackName()
}

// ToEnvMetadata inspects an environment cloudformation stack and constructs an environment
// struct out of it (including resources like ECR Repo).
func (e *BootstrapEnv) ToEnvMetadata(stack *cloudformation.Stack) (*config.Environment, error) {
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

func (e *Env) cdnConfig() *template.CDNConfig {
	if e.in.Mft == nil || !e.in.Mft.CDNEnabled() {
		return nil
	}
	mftConfig := e.in.Mft.CDNConfig.Config
	config := &template.CDNConfig{
		ImportedCertificate: mftConfig.Certificate,
		TerminateTLS:        aws.BoolValue(mftConfig.TerminateTLS),
	}
	if !mftConfig.Static.IsEmpty() {
		config.Static = &template.CDNStaticAssetConfig{
			ImportedBucket: mftConfig.Static.Location,
			Path:           mftConfig.Static.Path,
			Alias:          mftConfig.Static.Alias,
		}
	}
	return config
}

func (e *Env) publicHTTPConfig() template.PublicHTTPConfig {
	return template.PublicHTTPConfig{
		HTTPConfig: template.HTTPConfig{
			ImportedCertARNs: e.importPublicCertARNs(),
			SSLPolicy:        e.getPublicSSLPolicy(),
		},
		PublicALBSourceIPs: e.in.PublicALBSourceIPs,
		CIDRPrefixListIDs:  e.in.CIDRPrefixListIDs,
		ELBAccessLogs:      convertELBAccessLogsConfig(e.in.Mft),
	}
}

func (e *Env) privateHTTPConfig() template.PrivateHTTPConfig {
	return template.PrivateHTTPConfig{
		HTTPConfig: template.HTTPConfig{
			ImportedCertARNs: e.importPrivateCertARNs(),
			SSLPolicy:        e.getPrivateSSLPolicy(),
		},
		CustomALBSubnets: e.internalALBSubnets(),
	}
}

func (e *Env) vpcConfig() (template.VPCConfig, error) {
	securityGroupConfig, err := convertEnvSecurityGroupCfg(e.in.Mft)
	if err != nil {
		return template.VPCConfig{}, err
	}
	flowLogs, err := convertFlowLogsConfig(e.in.Mft)
	if err != nil {
		return template.VPCConfig{}, err
	}
	return template.VPCConfig{
		Imported:            e.importVPC(),
		Managed:             e.managedVPC(),
		AllowVPCIngress:     e.in.Mft.HTTPConfig.Private.HasVPCIngress(),
		SecurityGroupConfig: securityGroupConfig,
		FlowLogs:            flowLogs,
	}, nil
}

func (e *Env) importVPC() *template.ImportVPC {
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

func (e *Env) managedVPC() template.ManagedVPC {
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

func (e *Env) telemetryConfig() *template.Telemetry {
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

func (e *Env) importPublicCertARNs() []string {
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

func (e *Env) importPrivateCertARNs() []string {
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

func (e *Env) internalALBSubnets() []string {
	// If a manifest is present, it is the only place we look.
	if e.in.Mft != nil {
		return e.in.Mft.HTTPConfig.Private.InternalALBSubnets
	}
	// Fallthrough to SSM config.
	return e.in.InternalALBSubnets
}

func (e *Env) getPublicSSLPolicy() *string {
	return e.in.Mft.EnvironmentConfig.HTTPConfig.Public.SSLPolicy
}

func (e *Env) getPrivateSSLPolicy() *string {
	return e.in.Mft.EnvironmentConfig.HTTPConfig.Private.SSLPolicy
}
