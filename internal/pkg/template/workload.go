// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go/service/ecs"
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
	backendSvcTplName   = "backend"
	scheduledJobTplName = "scheduled-job"
)

// Constants for workload options.
const (
	// AWS VPC networking configuration.
	EnablePublicIP          = "ENABLED"
	DisablePublicIP         = "DISABLED"
	PublicSubnetsPlacement  = "PublicSubnets"
	PrivateSubnetsPlacement = "PrivateSubnets"
)

var (
	// Template names under "workloads/partials/cf/".
	partialsWorkloadCFTemplateNames = []string{
		"loggroup",
		"envvars",
		"secrets",
		"executionrole",
		"taskrole",
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
		"env-controller",
		"mount-points",
		"volumes",
		"image-overrides",
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
	Name        *string
	Image       *string
	Essential   *bool
	Port        *string
	Protocol    *string
	CredsParam  *string
	Variables   map[string]string
	Secrets     map[string]string
	MountPoints []*MountPoint
}

// StorageOpts holds data structures for rendering Volumes and Mount Points
type StorageOpts struct {
	Volumes           []*Volume
	MountPoints       []*MountPoint
	EFSPerms          []*EFSPermission
	ManagedVolumeInfo *ManagedVolumeCreationInfo // Used for delegating CreationInfo for Copilot-managed EFS.
}

// EFSPermission holds information needed to render an IAM policy statement.
type EFSPermission struct {
	FilesystemID  *string
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
	Name *string
	UID  *string
	GID  *string
}

type EFSVolumeConfiguration struct {
	// EFSVolumeConfiguration
	Filesystem    *string
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
	SecretOptions  map[string]string
	ConfigFile     *string
}

// HTTPHealthCheckOpts holds configuration that's needed for HTTP Health Check.
type HTTPHealthCheckOpts struct {
	HealthCheckPath    string
	HealthyThreshold   *int64
	UnhealthyThreshold *int64
	Interval           *int64
	Timeout            *int64
}

// AutoscalingOpts holds configuration that's needed for Auto Scaling.
type AutoscalingOpts struct {
	MinCapacity  *int
	MaxCapacity  *int
	CPU          *float64
	Memory       *float64
	Requests     *float64
	ResponseTime *float64
}

// ExecuteCommandOpts holds configuration that's needed for ECS Execute Command.
type ExecuteCommandOpts struct{}

// StateMachineOpts holds configuration needed for State Machine retries and timeout.
type StateMachineOpts struct {
	Timeout *int
	Retries *int
}

// NetworkOpts holds AWS networking configuration for the workloads.
type NetworkOpts struct {
	AssignPublicIP string
	SubnetsType    string
	SecurityGroups []string
}

func defaultNetworkOpts() *NetworkOpts {
	return &NetworkOpts{
		AssignPublicIP: EnablePublicIP,
		SubnetsType:    PublicSubnetsPlacement,
	}
}

// WorkloadOpts holds optional data that can be provided to enable features in a workload stack template.
type WorkloadOpts struct {
	// Additional options that are common between **all** workload templates.
	Variables      map[string]string
	Secrets        map[string]string
	NestedStack    *WorkloadNestedStackOpts // Outputs from nested stacks such as the addons stack.
	Sidecars       []*SidecarOpts
	LogConfig      *LogConfigOpts
	Autoscaling    *AutoscalingOpts
	Storage        *StorageOpts
	Network        *NetworkOpts
	ExecuteCommand *ExecuteCommandOpts
	EntryPoint     []string
	Command        []string
	DomainAlias    string

	// Additional options for service templates.
	WorkloadType        string
	HealthCheck         *ecs.HealthCheck
	HTTPHealthCheck     HTTPHealthCheckOpts
	AllowedSourceIps    []string
	RulePriorityLambda  string
	DesiredCountLambda  string
	EnvControllerLambda string

	// Additional options for job templates.
	ScheduleExpression string
	StateMachine       *StateMachineOpts
}

// ParseLoadBalancedWebService parses a load balanced web service's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseLoadBalancedWebService(data WorkloadOpts) (*Content, error) {
	if data.Network == nil {
		data.Network = defaultNetworkOpts()
	}
	return t.parseSvc(lbWebSvcTplName, data, withSvcParsingFuncs())
}

// ParseBackendService parses a backend service's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseBackendService(data WorkloadOpts) (*Content, error) {
	if data.Network == nil {
		data.Network = defaultNetworkOpts()
	}
	return t.parseSvc(backendSvcTplName, data, withSvcParsingFuncs())
}

// ParseScheduledJob parses a scheduled job's Cloudformation Template
func (t *Template) ParseScheduledJob(data WorkloadOpts) (*Content, error) {
	if data.Network == nil {
		data.Network = defaultNetworkOpts()
	}
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
			"toSnakeCase":         ToSnakeCaseFunc,
			"hasSecrets":          hasSecrets,
			"fmtSlice":            FmtSliceFunc,
			"quoteSlice":          QuotePSliceFunc,
			"randomUUID":          randomUUIDFunc,
			"jsonMountPoints":     generateMountPointJSON,
			"envControllerParams": envControllerParameters,
		})
	}
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
		parameters = append(parameters, "ALBWorkloads,") // YAML needs the comma separator; okay if trailing.
	}
	if o.Network.SubnetsType == PrivateSubnetsPlacement {
		parameters = append(parameters, "NATWorkloads,") // YAML needs the comma separator; okay if trailing.
	}
	return parameters
}
