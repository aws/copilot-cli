// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"slices"
	"strconv"
	"text/template"

	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/dustin/go-humanize/english"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go/aws"
)

// Constants for template paths.
const (
	// Paths of workload cloudformation templates under templates/workloads/.
	fmtWkldCFTemplatePath         = "workloads/%s/%s/cf.yml"
	fmtWkldPartialsCFTemplatePath = "workloads/partials/cf/%s.yml"

	// Directories under templates/workloads/.
	servicesDirName = "services"
	jobDirName      = "jobs"

	// Names of workload templates.
	lbWebSvcTplName     = "lb-web"
	rdWebSvcTplName     = "rd-web"
	backendSvcTplName   = "backend"
	workerSvcTplName    = "worker"
	staticSiteTplName   = "static-site"
	scheduledJobTplName = "scheduled-job"
)

// Constants for workload options.
const (
	// AWS VPC networking configuration.
	EnablePublicIP          = "ENABLED"
	DisablePublicIP         = "DISABLED"
	PublicSubnetsPlacement  = "PublicSubnets"
	PrivateSubnetsPlacement = "PrivateSubnets"

	// RuntimePlatform configuration.
	OSLinux                 = "LINUX"
	OSWindowsServerFull     = OSWindowsServer2019Full // Alias 2019 as Default WindowsSever Full platform.
	OSWindowsServerCore     = OSWindowsServer2019Core // Alias 2019 as Default WindowsSever Core platform.
	OSWindowsServer2019Full = "WINDOWS_SERVER_2019_FULL"
	OSWindowsServer2019Core = "WINDOWS_SERVER_2019_CORE"
	OSWindowsServer2022Full = "WINDOWS_SERVER_2022_FULL"
	OSWindowsServer2022Core = "WINDOWS_SERVER_2022_CORE"

	ArchX86   = "X86_64"
	ArchARM64 = "ARM64"
)

// Constants for ARN options.
const (
	snsARNPattern = "arn:%s:sns:%s:%s:%s-%s-%s-%s"
)

// Constants for stack resource logical IDs
const (
	LogicalIDHTTPListenerRuleWithDomain = "HTTPListenerRuleWithDomain"
)

const (
	// NoExposedContainerPort indicates no port should be exposed for the service container.
	NoExposedContainerPort = "-1"
)

var (
	// Template names under "workloads/partials/cf/".
	partialsWorkloadCFTemplateNames = []string{
		"loggroup",
		"envvars-container",
		"envvars-common",
		"secrets",
		"executionrole",
		"taskrole",
		"workload-container",
		"fargate-taskdef-base-properties",
		"service-base-properties",
		"servicediscovery",
		"addons",
		"sidecars",
		"logconfig",
		"autoscaling",
		"eventrule",
		"state-machine",
		"state-machine-definition.json",
		"efs-access-point",
		"https-listener",
		"http-listener",
		"env-controller",
		"mount-points",
		"variables",
		"volumes",
		"image-overrides",
		"instancerole",
		"accessrole",
		"publish",
		"subscribe",
		"nlb",
		"vpc-connector",
		"alb",
		"rollback-alarms",
		"imported-alb-resources",
	}

	// Operating systems to determine Fargate platform versions.
	osFamiliesForPV100 = []string{
		OSWindowsServer2019Full, OSWindowsServer2019Core, OSWindowsServer2022Full, OSWindowsServer2022Core,
	}
)

// WorkloadNestedStackOpts holds configuration that's needed if the workload stack has a nested stack.
type WorkloadNestedStackOpts struct {
	StackName string

	VariableOutputs      []string
	SecretOutputs        []string
	PolicyOutputs        []string
	SecurityGroupOutputs []string
}

// SidecarOpts holds configuration that's needed if the service has sidecar containers.
type SidecarOpts struct {
	Name         string
	Image        *string
	Essential    *bool
	CredsParam   *string
	Variables    map[string]Variable
	Secrets      map[string]Secret
	Storage      SidecarStorageOpts
	DockerLabels map[string]string
	DependsOn    map[string]string
	EntryPoint   []string
	Command      []string
	HealthCheck  *ContainerHealthCheck
	PortMappings []*PortMapping
}

// PortMapping holds container port mapping configuration.
type PortMapping struct {
	Protocol      string
	ContainerPort uint16
	ContainerName string
}

// SidecarStorageOpts holds data structures for rendering Mount Points inside of a sidecar.
type SidecarStorageOpts struct {
	MountPoints []*MountPoint
}

// StorageOpts holds data structures for rendering Volumes and Mount Points
type StorageOpts struct {
	Ephemeral         *int
	ReadonlyRootFS    *bool
	Volumes           []*Volume
	MountPoints       []*MountPoint
	EFSPerms          []*EFSPermission
	ManagedVolumeInfo *ManagedVolumeCreationInfo // Used for delegating CreationInfo for Copilot-managed EFS.
}

// requiresEFSCreation returns true if managed volume information is specified; false otherwise.
func (s *StorageOpts) requiresEFSCreation() bool {
	return s.ManagedVolumeInfo != nil
}

// EFSPermission holds information needed to render an IAM policy statement.
type EFSPermission struct {
	FilesystemID  FileSystemID
	Write         bool
	AccessPointID *string
}

// MountPoint holds information needed to render a MountPoint in a containerdefinition.
type MountPoint struct {
	ContainerPath *string
	ReadOnly      *bool
	SourceVolume  *string
}

// Volume contains fields that render a volume, its name, and EFSVolumeConfiguration
type Volume struct {
	Name *string

	EFS *EFSVolumeConfiguration
}

// ManagedVolumeCreationInfo holds information about how to create Copilot-managed access points.
type ManagedVolumeCreationInfo struct {
	Name    *string
	DirName *string
	UID     *uint32
	GID     *uint32
}

// EFSVolumeConfiguration contains information about how to specify externally managed file systems.
type EFSVolumeConfiguration struct {
	// EFSVolumeConfiguration
	Filesystem    FileSystemID
	RootDirectory *string // "/" or empty are equivalent

	// Authorization Config
	AccessPointID *string
	IAM           *string // ENABLED or DISABLED
}

// LogConfigOpts holds configuration that's needed if the service is configured with Firelens to route
// its logs.
type LogConfigOpts struct {
	Image          *string
	Destination    map[string]string
	EnableMetadata *string
	SecretOptions  map[string]Secret
	ConfigFile     *string
	Variables      map[string]Variable
	Secrets        map[string]Secret
}

// StrconvUint16 returns string converted from uint16.
func StrconvUint16(val uint16) string {
	return strconv.FormatUint(uint64(val), 10)
}

// HTTPHealthCheckOpts holds configuration that's needed for HTTP Health Check.
type HTTPHealthCheckOpts struct {
	// Fields with defaults always set.
	HealthCheckPath string
	GracePeriod     int64

	// Optional.
	Port                string
	SuccessCodes        string
	HealthyThreshold    *int64
	UnhealthyThreshold  *int64
	Interval            *int64
	Timeout             *int64
	DeregistrationDelay *int64
}

type importable interface {
	RequiresImport() bool
}

type importableValue interface {
	importable
	Value() string
}

// FileSystemID represnts the EFS FilesystemID.
type FileSystemID importableValue

// PlainFileSystemID returns a EFS FilesystemID that is a plain string value.
func PlainFileSystemID(value string) FileSystemID {
	return plainFileSystemID(value)
}

// ImportedFileSystemID returns a EFS FilesystemID that is imported from a stack.
func ImportedFileSystemID(value string) FileSystemID {
	return importedFileSystemID(value)
}

type plainFileSystemID string

// RequiresImport returns false for a plain EFS FilesystemID.
func (fs plainFileSystemID) RequiresImport() bool {
	return false
}

// Value returns the plain string value of the plain EFS FilesystemID.
func (fs plainFileSystemID) Value() string {
	return string(fs)
}

type importedFileSystemID string

// RequiresImport returns true for a imported EFS FilesystemID.
func (fs importedFileSystemID) RequiresImport() bool {
	return true
}

// Value returns the name of the import that will be the value of the EFS Filesystem ID.
func (fs importedFileSystemID) Value() string {
	return string(fs)
}

// Variable represents the value of an environment variable.
type Variable importableValue

// ImportedVariable returns a Variable that should be imported from a stack.
func ImportedVariable(name string) Variable {
	return importedEnvVar(name)
}

// PlainVariable returns a Variable that is a plain string value.
func PlainVariable(value string) Variable {
	return plainEnvVar(value)
}

type plainEnvVar string

// RequiresImport returns false for a plain string environment variable.
func (v plainEnvVar) RequiresImport() bool {
	return false
}

// Value returns the plain string value of the environment variable.
func (v plainEnvVar) Value() string {
	return string(v)
}

type importedEnvVar string

// RequiresImport returns true for an imported environment variable.
func (v importedEnvVar) RequiresImport() bool {
	return true
}

// Value returns the name of the import that will be the value of the environment variable.
func (v importedEnvVar) Value() string {
	return string(v)
}

type importableSubValueFrom interface {
	importable
	RequiresSub() bool
	ValueFrom() string
}

// A Secret represents an SSM or SecretsManager secret that can be rendered in CloudFormation.
type Secret importableSubValueFrom

// plainSSMOrSecretARN is a Secret stored that can be referred by an SSM Parameter Name or a secret ARN.
type plainSSMOrSecretARN struct {
	value string
}

// RequiresSub returns true if the secret should be populated in CloudFormation with !Sub.
func (s plainSSMOrSecretARN) RequiresSub() bool {
	return false
}

// RequiresImport returns true if the secret should be imported from other CloudFormation stack.
func (s plainSSMOrSecretARN) RequiresImport() bool {
	return false
}

// ValueFrom returns the plain string value of the secret.
func (s plainSSMOrSecretARN) ValueFrom() string {
	return s.value
}

// SecretFromPlainSSMOrARN returns a Secret that refers to an SSM parameter or a secret ARN.
func SecretFromPlainSSMOrARN(value string) plainSSMOrSecretARN {
	return plainSSMOrSecretARN{
		value: value,
	}
}

// importedSSMorSecretARN is a Secret that can be referred by the name of the import value from env addon or an arbitary CloudFormation stack.
type importedSSMorSecretARN struct {
	value string
}

// RequiresSub returns true if the secret should be populated in CloudFormation with !Sub.
func (s importedSSMorSecretARN) RequiresSub() bool {
	return false
}

// RequiresImport returns true if the secret should be imported from env addon or an arbitary CloudFormation stack.
func (s importedSSMorSecretARN) RequiresImport() bool {
	return true
}

// ValueFrom returns the name of the import value of the Secret.
func (s importedSSMorSecretARN) ValueFrom() string {
	return s.value
}

// SecretFromImportedSSMOrARN returns a Secret that refers to imported name of SSM parameter or a secret ARN.
func SecretFromImportedSSMOrARN(value string) importedSSMorSecretARN {
	return importedSSMorSecretARN{
		value: value,
	}
}

// secretsManagerName is a Secret that can be referred by a SecretsManager secret name.
type secretsManagerName struct {
	value string
}

// RequiresSub returns true if the secret should be populated in CloudFormation with !Sub.
func (s secretsManagerName) RequiresSub() bool {
	return true
}

// RequiresImport returns true if the secret should be imported from other CloudFormation stack.
func (s secretsManagerName) RequiresImport() bool {
	return false
}

// ValueFrom returns the resource ID of the SecretsManager secret for populating the ARN.
func (s secretsManagerName) ValueFrom() string {
	return fmt.Sprintf("secret:%s", s.value)
}

// Service returns the name of the SecretsManager service for populating the ARN.
func (s secretsManagerName) Service() string {
	return secretsmanager.ServiceName
}

// SecretFromSecretsManager returns a Secret that refers to SecretsManager secret name.
func SecretFromSecretsManager(value string) secretsManagerName {
	return secretsManagerName{
		value: value,
	}
}

// NetworkLoadBalancerListener holds configuration that's need for a Network Load Balancer listener.
type NetworkLoadBalancerListener struct {
	// The port and protocol that the Network Load Balancer listens to.
	Port     string
	Protocol string

	// The target container and port to which the traffic is routed to from the Network Load Balancer.
	TargetContainer string
	TargetPort      string

	SSLPolicy *string // The SSL policy applied when using TLS protocol.

	Stickiness          *bool
	HealthCheck         NLBHealthCheck
	DeregistrationDelay *int64
}

// NLBHealthCheck holds configuration for Network Load Balancer health check.
type NLBHealthCheck struct {
	Port               string // The port to which health check requests made from Network Load Balancer are routed to.
	HealthyThreshold   *int64
	UnhealthyThreshold *int64
	Timeout            *int64
	Interval           *int64
	GracePeriod        *int64
}

// NetworkLoadBalancer holds configuration that's needed for a Network Load Balancer.
type NetworkLoadBalancer struct {
	Listener            []NetworkLoadBalancerListener
	MainContainerPort   string
	CertificateRequired bool
	Aliases             []string
}

// ALBListenerRule holds configuration that's needed for an Application Load Balancer listener rule.
type ALBListenerRule struct {
	// The path that the Application Load Balancer listens to.
	Path string
	// The target container and port to which the traffic is routed to from the Application Load Balancer.
	TargetContainer string
	TargetPort      string

	Aliases             []string
	AllowedSourceIps    []string
	Stickiness          string
	HTTPHealthCheck     HTTPHealthCheckOpts
	HTTPVersion         string
	RedirectToHTTPS     bool // Only relevant if HTTPSListener is true.
	DeregistrationDelay *int64
}

// ALBListener holds configuration that's needed for an Application Load Balancer Listener.
type ALBListener struct {
	Rules             []ALBListenerRule
	HostedZoneAliases AliasesForHostedZone
	IsHTTPS           bool // True if the listener listening on port 443.
	MainContainerPort string
}

// Aliases return all the unique aliases specified across all the routing rules in ALB.
func (cfg *ALBListener) Aliases() []string {
	var uniqueAliases []string
	seen := make(map[string]struct{})
	exists := struct{}{}
	for _, rule := range cfg.Rules {
		for _, entry := range rule.Aliases {
			if _, value := seen[entry]; !value {
				uniqueAliases = append(uniqueAliases, entry)
				seen[entry] = exists
			}
		}
	}
	return uniqueAliases
}

// RulePaths returns a slice consisting of all the routing paths mentioned across multiple listener rules.
func (cfg *ALBListener) RulePaths() []string {
	var rulePaths []string
	for _, rule := range cfg.Rules {
		rulePaths = append(rulePaths, rule.Path)
	}
	return rulePaths
}

// ServiceConnectOpts defines the options for service connect.
// If Client is false, logically Server must be nil.
type ServiceConnectOpts struct {
	Server *ServiceConnectServer
	Client bool
}

// ServiceConnectServer defines the container name and port which a service routes Service Connect through.
type ServiceConnectServer struct {
	Name  string
	Port  string
	Alias string
}

// AdvancedCount holds configuration for autoscaling and capacity provider
// parameters.
type AdvancedCount struct {
	Spot        *int
	Autoscaling *AutoscalingOpts
	Cps         []*CapacityProviderStrategy
}

// ContainerHealthCheck holds configuration for container health check.
type ContainerHealthCheck struct {
	Command     []string
	Interval    *int64
	Retries     *int64
	StartPeriod *int64
	Timeout     *int64
}

// CapacityProviderStrategy holds the configuration needed for a
// CapacityProviderStrategyItem on a Service
type CapacityProviderStrategy struct {
	Base             *int
	Weight           *int
	CapacityProvider string
}

// Cooldown holds configuration needed for autoscaling cooldown fields.
type Cooldown struct {
	ScaleInCooldown  *float64
	ScaleOutCooldown *float64
}

// AutoscalingOpts holds configuration that's needed for Auto Scaling.
type AutoscalingOpts struct {
	MinCapacity        *int
	MaxCapacity        *int
	CPU                *float64
	Memory             *float64
	Requests           *float64
	ResponseTime       *float64
	CPUCooldown        Cooldown
	MemCooldown        Cooldown
	ReqCooldown        Cooldown
	RespTimeCooldown   Cooldown
	QueueDelayCooldown Cooldown
	QueueDelay         *AutoscalingQueueDelayOpts
}

// AliasesForHostedZone maps hosted zone IDs to aliases that belong to it.
type AliasesForHostedZone map[string][]string

// AutoscalingQueueDelayOpts holds configuration to scale SQS queues.
type AutoscalingQueueDelayOpts struct {
	AcceptableBacklogPerTask int
}

// ObservabilityOpts holds configurations for observability.
type ObservabilityOpts struct {
	Tracing string // The name of the vendor used for tracing.
}

// DeploymentConfigurationOpts holds configuration for rolling deployments.
type DeploymentConfigurationOpts struct {
	// The lower limit on the number of tasks that should be running during a service deployment or when a container instance is draining.
	MinHealthyPercent int
	// The upper limit on the number of tasks that should be running during a service deployment or when a container instance is draining.
	MaxPercent int
	Rollback   RollingUpdateRollbackConfig
}

// RollingUpdateRollbackConfig holds config for rollback alarms.
type RollingUpdateRollbackConfig struct {
	AlarmNames []string // Names of existing alarms.

	// Custom alarms to create.
	CPUUtilization    *float64
	MemoryUtilization *float64
	MessagesDelayed   *int
}

// HasRollbackAlarms returns true if the client is using ABR.
func (cfg RollingUpdateRollbackConfig) HasRollbackAlarms() bool {
	return len(cfg.AlarmNames) > 0 || cfg.HasCustomAlarms()
}

// HasCustomAlarms returns true if the client is using Copilot-generated alarms for alarm-based rollbacks.
func (cfg RollingUpdateRollbackConfig) HasCustomAlarms() bool {
	return cfg.CPUUtilization != nil || cfg.MemoryUtilization != nil || cfg.MessagesDelayed != nil
}

// TruncateAlarmName ensures that alarm names don't exceed the 255 character limit.
func (cfg RollingUpdateRollbackConfig) TruncateAlarmName(app, env, svc, alarmType string) string {
	if len(app)+len(env)+len(svc)+len(alarmType) <= 255 {
		return fmt.Sprintf("%s-%s-%s-%s", app, env, svc, alarmType)
	}
	maxSubstringLength := (255 - len(alarmType) - 3) / 3
	return fmt.Sprintf("%s-%s-%s-%s", app[:maxSubstringLength], env[:maxSubstringLength], svc[:maxSubstringLength], alarmType)
}

// ExecuteCommandOpts holds configuration that's needed for ECS Execute Command.
type ExecuteCommandOpts struct{}

// StateMachineOpts holds configuration needed for State Machine retries and timeout.
type StateMachineOpts struct {
	Timeout *int
	Retries *int
}

// PublishOpts holds configuration needed if the service has publishers.
type PublishOpts struct {
	Topics []*Topic
}

// Topic holds information needed to render a SNSTopic in a container definition.
type Topic struct {
	Name            *string
	FIFOTopicConfig *FIFOTopicConfig

	Region    string
	Partition string
	AccountID string
	App       string
	Env       string
	Svc       string
}

// FIFOTopicConfig holds configuration needed if the topic is FIFO.
type FIFOTopicConfig struct {
	ContentBasedDeduplication *bool
}

// SubscribeOpts holds configuration needed if the service has subscriptions.
type SubscribeOpts struct {
	Topics []*TopicSubscription
	Queue  *SQSQueue
}

// HasTopicQueues returns true if any individual subscription has a dedicated queue.
func (s *SubscribeOpts) HasTopicQueues() bool {
	for _, t := range s.Topics {
		if t.Queue != nil {
			return true
		}
	}
	return false
}

// TopicSubscription holds information needed to render a SNS Topic Subscription in a container definition.
type TopicSubscription struct {
	Name         *string
	Service      *string
	FilterPolicy *string
	Queue        *SQSQueue
}

// SQSQueue holds information needed to render a SQS Queue in a container definition.
type SQSQueue struct {
	Retention       *int64
	Delay           *int64
	Timeout         *int64
	DeadLetter      *DeadLetterQueue
	FIFOQueueConfig *FIFOQueueConfig
}

// FIFOQueueConfig holds information needed to render a FIFO SQS Queue in a container definition.
type FIFOQueueConfig struct {
	FIFOThroughputLimit       *string
	ContentBasedDeduplication *bool
	DeduplicationScope        *string
}

// DeadLetterQueue holds information needed to render a dead-letter SQS Queue in a container definition.
type DeadLetterQueue struct {
	Tries *uint16
}

// NetworkOpts holds AWS networking configuration for the workloads.
type NetworkOpts struct {
	SecurityGroups []SecurityGroup
	AssignPublicIP string
	// SubnetsType and SubnetIDs are mutually exclusive. They won't be set together.
	SubnetsType              string
	SubnetIDs                []string
	DenyDefaultSecurityGroup bool
}

// SecurityGroup represents the ID of an additional security group associated with the tasks.
type SecurityGroup importableValue

// PlainSecurityGroup returns a SecurityGroup that is a plain string value.
func PlainSecurityGroup(value string) SecurityGroup {
	return plainSecurityGroup(value)
}

// ImportedSecurityGroup returns a SecurityGroup that should be imported from a stack.
func ImportedSecurityGroup(name string) SecurityGroup {
	return importedSecurityGroup(name)
}

type plainSecurityGroup string

// RequiresImport returns false for a plain string SecurityGroup.
func (sg plainSecurityGroup) RequiresImport() bool {
	return false
}

// Value returns the plain string value of the SecurityGroup.
func (sg plainSecurityGroup) Value() string {
	return string(sg)
}

type importedSecurityGroup string

// RequiresImport returns true for an imported SecurityGroup.
func (sg importedSecurityGroup) RequiresImport() bool {
	return true
}

// Value returns the name of the import that will be the value of the SecurityGroup.
func (sg importedSecurityGroup) Value() string {
	return string(sg)
}

// RuntimePlatformOpts holds configuration needed for Platform configuration.
type RuntimePlatformOpts struct {
	OS   string
	Arch string
}

// IsDefault returns true if the platform matches the default docker image platform of "linux/amd64".
func (p RuntimePlatformOpts) IsDefault() bool {
	if p.isEmpty() {
		return true
	}
	if p.OS == OSLinux && p.Arch == ArchX86 {
		return true
	}
	return false
}

// Version returns the Fargate platform version based on the selected os family.
func (p RuntimePlatformOpts) Version() string {
	for _, os := range osFamiliesForPV100 {
		if p.OS == os {
			return "1.0.0"
		}
	}
	return "LATEST"
}

func (p RuntimePlatformOpts) isEmpty() bool {
	return p.OS == "" && p.Arch == ""
}

// S3ObjectLocation represents an object stored in an S3 bucket.
type S3ObjectLocation struct {
	Bucket string // Name of the bucket.
	Key    string // Key of the object.
}

// WorkloadOpts holds optional data that can be provided to enable features in a workload stack template.
type WorkloadOpts struct {
	AppName            string
	EnvName            string
	WorkloadName       string
	SerializedManifest string // Raw manifest file used to deploy the workload.
	EnvVersion         string
	Version            string

	// Configuration for the main container.
	PortMappings []*PortMapping
	Variables    map[string]Variable
	Secrets      map[string]Secret
	EntryPoint   []string
	Command      []string
	ImportedALB  *ImportedALB

	// Additional options that are common between **all** workload templates.
	Tags                     map[string]string        // Used by App Runner workloads to tag App Runner service resources
	NestedStack              *WorkloadNestedStackOpts // Outputs from nested stacks such as the addons stack.
	AddonsExtraParams        string                   // Additional user defined Parameters for the addons stack.
	Sidecars                 []*SidecarOpts
	LogConfig                *LogConfigOpts
	Autoscaling              *AutoscalingOpts
	CapacityProviders        []*CapacityProviderStrategy
	DesiredCountOnSpot       *int
	Storage                  *StorageOpts
	Network                  NetworkOpts
	ExecuteCommand           *ExecuteCommandOpts
	Platform                 RuntimePlatformOpts
	DockerLabels             map[string]string
	DependsOn                map[string]string
	Publish                  *PublishOpts
	ServiceDiscoveryEndpoint string
	ALBEnabled               bool
	CredentialsParameter     string
	PermissionsBoundary      string

	// Additional options for service templates.
	WorkloadType            string
	HealthCheck             *ContainerHealthCheck
	GracePeriod             *int64
	NLB                     *NetworkLoadBalancer
	ALBListener             *ALBListener
	DeploymentConfiguration DeploymentConfigurationOpts
	ServiceConnectOpts      ServiceConnectOpts

	// Custom Resources backed by Lambda functions.
	CustomResources map[string]S3ObjectLocation

	// Additional options for job templates.
	ScheduleExpression string
	StateMachine       *StateMachineOpts

	// Additional options for request driven web service templates.
	StartCommand         *string
	EnableHealthCheck    bool
	Observability        ObservabilityOpts
	Private              bool
	AppRunnerVPCEndpoint *string
	Count                *string

	// Input needed for the custom resource that adds a custom domain to the service.
	Alias                *string
	AWSSDKLayer          *string
	AppDNSDelegationRole *string
	AppDNSName           *string

	// Additional options for worker service templates.
	Subscribe *SubscribeOpts

	// Additional options for static site template.
	AssetMappingFileBucket string
	AssetMappingFilePath   string
	StaticSiteAlias        string
	StaticSiteCert         string
}

// HealthCheckProtocol returns the protocol for the Load Balancer health check,
// or an empty string if it shouldn't be configured, defaulting to the
// target protocol. (which is what happens, even if it isn't documented as such :))
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-elasticloadbalancingv2-targetgroup.html#cfn-elasticloadbalancingv2-targetgroup-healthcheckprotocol
func (lr ALBListenerRule) HealthCheckProtocol() string {
	switch {
	case lr.HTTPHealthCheck.Port == "443":
		return "HTTPS"
	case lr.TargetPort == "443" && lr.HTTPHealthCheck.Port == "":
		return "HTTPS"
	case lr.TargetPort == "443" && lr.HTTPHealthCheck.Port != "443":
		// for backwards compatability, only set HTTP if target
		// container is https but the specified health check port is not
		return "HTTP"
	}
	return ""
}

// ImportedALB holds the fields to import an existing ALB.
type ImportedALB struct {
	Name         string
	ARN          string
	DNSName      string
	HostedZoneID string

	Listeners      []LBListener
	SecurityGroups []LBSecurityGroup
}

// LBListener struct represents the listener of a load balancer. // TODO(jwh): instead, reuse ALBListener
type LBListener struct {
	ARN      string
	Port     int64
	Protocol string
}

// LBSecurityGroup struct represents the security group of a load balancer.
type LBSecurityGroup struct {
	ID string
}

// HTTPListenerARN returns the listener ARN if the protocol is HTTP.
func (alb *ImportedALB) HTTPListenerARN() string {
	for _, listener := range alb.Listeners {
		if listener.Protocol == "HTTP" {
			return listener.ARN
		}
	}
	return ""
}

// HTTPSListenerARN returns the listener ARN if the protocol is HTTPS.
func (alb *ImportedALB) HTTPSListenerARN() string {
	for _, listener := range alb.Listeners {
		if listener.Protocol == "HTTPS" {
			return listener.ARN
		}
	}
	return ""
}

// ParseLoadBalancedWebService parses a load balanced web service's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseLoadBalancedWebService(data WorkloadOpts) (*Content, error) {
	return t.parseSvc(lbWebSvcTplName, data, withSvcParsingFuncs())
}

// ParseRequestDrivenWebService parses a request-driven web service's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseRequestDrivenWebService(data WorkloadOpts) (*Content, error) {
	return t.parseSvc(rdWebSvcTplName, data, withSvcParsingFuncs())
}

// ParseBackendService parses a backend service's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseBackendService(data WorkloadOpts) (*Content, error) {
	return t.parseSvc(backendSvcTplName, data, withSvcParsingFuncs())
}

// ParseWorkerService parses a worker service's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseWorkerService(data WorkloadOpts) (*Content, error) {
	return t.parseSvc(workerSvcTplName, data, withSvcParsingFuncs())
}

// ParseStaticSite parses a static site service's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseStaticSite(data WorkloadOpts) (*Content, error) {
	return t.parseSvc(staticSiteTplName, data, withSvcParsingFuncs())
}

// ParseScheduledJob parses a scheduled job's Cloudformation Template
func (t *Template) ParseScheduledJob(data WorkloadOpts) (*Content, error) {
	return t.parseJob(scheduledJobTplName, data, withSvcParsingFuncs())
}

// parseSvc parses a service's CloudFormation template with the specified data object and returns its content.
func (t *Template) parseSvc(name string, data interface{}, options ...ParseOption) (*Content, error) {
	return t.parseWkld(name, servicesDirName, data, options...)
}

// parseJob parses a job's Cloudformation template with the specified data object and returns its content.
func (t *Template) parseJob(name string, data interface{}, options ...ParseOption) (*Content, error) {
	return t.parseWkld(name, jobDirName, data, options...)
}

func (t *Template) parseWkld(name, wkldDirName string, data interface{}, options ...ParseOption) (*Content, error) {
	tpl, err := t.parse("base", fmt.Sprintf(fmtWkldCFTemplatePath, wkldDirName, name), options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range partialsWorkloadCFTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtWkldPartialsCFTemplatePath, templateName), options...)
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
		return nil, fmt.Errorf("execute template %s with data %v: %w", name, data, err)
	}
	return &Content{buf}, nil
}

func withSvcParsingFuncs() ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(map[string]interface{}{
			"toSnakeCase":             ToSnakeCaseFunc,
			"hasSecrets":              hasSecrets,
			"fmtSlice":                FmtSliceFunc,
			"quoteSlice":              QuoteSliceFunc,
			"quote":                   strconv.Quote,
			"randomUUID":              randomUUIDFunc,
			"jsonMountPoints":         generateMountPointJSON,
			"jsonSNSTopics":           generateSNSJSON,
			"jsonQueueURIs":           generateQueueURIJSON,
			"envControllerParams":     envControllerParameters,
			"logicalIDSafe":           StripNonAlphaNumFunc,
			"wordSeries":              english.WordSeries,
			"pluralWord":              english.PluralWord,
			"contains":                slices.Contains[[]string, string],
			"requiresVPCConnector":    requiresVPCConnector,
			"strconvUint16":           StrconvUint16,
			"truncateWithHashPadding": truncateWithHashPadding,
		})
	}
}

func truncateWithHashPadding(s string, max, paddingLength int) string {
	if len(s) <= max {
		return s
	}
	h := sha256.New()
	h.Write([]byte(s))
	hash := fmt.Sprintf("%x", h.Sum(nil))
	return s[:max] + hash[:paddingLength]
}

func hasSecrets(opts WorkloadOpts) bool {
	if len(opts.Secrets) > 0 {
		return true
	}
	if opts.NestedStack != nil && (len(opts.NestedStack.SecretOutputs) > 0) {
		return true
	}
	return false
}

func randomUUIDFunc() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate random uuid: %w", err)
	}
	return id.String(), err
}

// envControllerParameters determines which parameters to include in the EnvController template.
func envControllerParameters(o WorkloadOpts) []string {
	parameters := []string{}
	if o.WorkloadType == "Load Balanced Web Service" {
		if o.ImportedALB == nil {
			if o.ALBEnabled {
				parameters = append(parameters, "ALBWorkloads,")
			}
			parameters = append(parameters, "Aliases,") // YAML needs the comma separator; resolved in EnvContr.
		}
	}
	if o.WorkloadType == "Backend Service" {
		if o.ALBEnabled {
			parameters = append(parameters, "InternalALBWorkloads,")
		}
	}
	if o.WorkloadType == "Request-Driven Web Service" {
		if o.Private && o.AppRunnerVPCEndpoint == nil {
			parameters = append(parameters, "AppRunnerPrivateWorkloads,")
		}
	}
	if o.Network.SubnetsType == PrivateSubnetsPlacement {
		parameters = append(parameters, "NATWorkloads,")
	}
	if o.Storage != nil && o.Storage.requiresEFSCreation() {
		parameters = append(parameters, "EFSWorkloads,")
	}
	return parameters
}

func requiresVPCConnector(o WorkloadOpts) bool {
	if o.WorkloadType != "Request-Driven Web Service" {
		return false
	}
	return len(o.Network.SubnetIDs) > 0 || o.Network.SubnetsType != ""
}

// ARN determines the arn for a topic using the SNSTopic name and account information
func (t Topic) ARN() string {
	return fmt.Sprintf(snsARNPattern, t.Partition, t.Region, t.AccountID, t.App, t.Env, t.Svc, aws.StringValue(t.Name))
}
