// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"gopkg.in/yaml.v3"
)

const (
	// Ignored resources
	rulePriorityFunction = "Custom::RulePriorityFunction"
	waitCondition        = "AWS::CloudFormation::WaitCondition"
	waitConditionHandle  = "AWS::CloudFormation::WaitConditionHandle"
)

const apprunnerServiceType = "AWS::AppRunner::Service"

// ConfigStoreSvc wraps methods of config store.
type ConfigStoreSvc interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
	ListServices(appName string) ([]*config.Workload, error)
	GetWorkload(appName string, name string) (*config.Workload, error)
	ListJobs(appName string) ([]*config.Workload, error)
}

// DeployedEnvServicesLister wraps methods of deploy store.
type DeployedEnvServicesLister interface {
	ListEnvironmentsDeployedTo(appName string, svcName string) ([]string, error)
	ListDeployedServices(appName string, envName string) ([]string, error)
	ListDeployedJobs(appName string, envName string) ([]string, error)
}

type ecsClient interface {
	TaskDefinition(app, env, svc string) (*awsecs.TaskDefinition, error)
}

type apprunnerClient interface {
	DescribeService(svcArn string) (*apprunner.Service, error)
}

type workloadStackDescriber interface {
	Params() (map[string]string, error)
	Outputs() (map[string]string, error)
	ServiceStackResources() ([]*stack.Resource, error)
	Manifest() ([]byte, error)
}

type ecsDescriber interface {
	workloadStackDescriber

	Platform() (*awsecs.ContainerPlatform, error)
	EnvVars() ([]*awsecs.ContainerEnvVar, error)
	Secrets() ([]*awsecs.ContainerSecret, error)
}

type apprunnerDescriber interface {
	workloadStackDescriber

	Service() (*apprunner.Service, error)
	ServiceARN() (string, error)
	ServiceURL() (string, error)
}

// serviceStackDescriber provides base functionality for retrieving info about a service.
type serviceStackDescriber struct {
	app     string
	service string
	env     string

	cfn  stackDescriber
	sess *session.Session

	// Cache variables.
	params         map[string]string
	outputs        map[string]string
	stackResources []*stack.Resource
}

// newServiceStackDescriber instantiates the core elements of a new service.
func newServiceStackDescriber(opt NewServiceConfig, env string) (*serviceStackDescriber, error) {
	environment, err := opt.ConfigStore.GetEnvironment(opt.App, env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", env, err)
	}
	sess, err := sessions.ImmutableProvider().FromRole(environment.ManagerRoleARN, environment.Region)
	if err != nil {
		return nil, err
	}
	return &serviceStackDescriber{
		app:     opt.App,
		service: opt.Svc,
		env:     env,

		cfn:  stack.NewStackDescriber(cfnstack.NameForService(opt.App, env, opt.Svc), sess),
		sess: sess,
	}, nil
}

// Params returns the parameters of the service stack.
func (d *serviceStackDescriber) Params() (map[string]string, error) {
	if d.params != nil {
		return d.params, nil
	}
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	d.params = descr.Parameters
	return descr.Parameters, nil
}

// Outputs returns the outputs of the service stack.
func (d *serviceStackDescriber) Outputs() (map[string]string, error) {
	if d.outputs != nil {
		return d.outputs, nil
	}
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	d.outputs = descr.Outputs
	return descr.Outputs, nil
}

// ServiceStackResources returns the filtered service stack resources created by CloudFormation.
func (d *serviceStackDescriber) ServiceStackResources() ([]*stack.Resource, error) {
	if len(d.stackResources) != 0 {
		return d.stackResources, nil
	}
	svcResources, err := d.cfn.Resources()
	if err != nil {
		return nil, err
	}
	var resources []*stack.Resource
	ignoredResources := map[string]bool{
		rulePriorityFunction: true,
		waitCondition:        true,
		waitConditionHandle:  true,
	}
	for _, svcResource := range svcResources {
		if !ignoredResources[svcResource.Type] {
			resources = append(resources, svcResource)
		}
	}
	d.stackResources = resources
	return resources, nil
}

// Manifest returns the contents of the manifest used to deploy a workload stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *serviceStackDescriber) Manifest() ([]byte, error) {
	tpl, err := d.cfn.StackMetadata()
	if err != nil {
		return nil, fmt.Errorf("retrieve stack metadata for %s-%s-%s: %w", d.app, d.env, d.service, err)
	}

	metadata := struct {
		Manifest string `yaml:"Manifest"`
	}{}
	if err := yaml.Unmarshal([]byte(tpl), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal Metadata.Manifest in stack %s-%s-%s: %v", d.app, d.env, d.service, err)
	}
	if len(strings.TrimSpace(metadata.Manifest)) == 0 {
		return nil, &ErrManifestNotFoundInTemplate{
			app:  d.app,
			env:  d.env,
			name: d.service,
		}
	}
	return []byte(metadata.Manifest), nil
}

type ecsServiceDescriber struct {
	*serviceStackDescriber
	ecsClient ecsClient
}

type appRunnerServiceDescriber struct {
	*serviceStackDescriber
	apprunnerClient apprunnerClient
}

// NewServiceConfig contains fields that initiates service describer struct.
type NewServiceConfig struct {
	App         string
	Svc         string
	ConfigStore ConfigStoreSvc

	EnableResources bool
	DeployStore     DeployedEnvServicesLister
}

func newECSServiceDescriber(opt NewServiceConfig, env string) (*ecsServiceDescriber, error) {
	stackDescriber, err := newServiceStackDescriber(opt, env)
	if err != nil {
		return nil, err
	}
	return &ecsServiceDescriber{
		serviceStackDescriber: stackDescriber,
		ecsClient:             ecs.New(stackDescriber.sess),
	}, nil
}

func newAppRunnerServiceDescriber(opt NewServiceConfig, env string) (*appRunnerServiceDescriber, error) {
	stackDescriber, err := newServiceStackDescriber(opt, env)
	if err != nil {
		return nil, err
	}

	return &appRunnerServiceDescriber{
		serviceStackDescriber: stackDescriber,
		apprunnerClient:       apprunner.New(stackDescriber.sess),
	}, nil
}

// EnvVars returns the environment variables of the task definition.
func (d *ecsServiceDescriber) EnvVars() ([]*awsecs.ContainerEnvVar, error) {
	taskDefinition, err := d.ecsClient.TaskDefinition(d.app, d.env, d.service)
	if err != nil {
		return nil, fmt.Errorf("describe task definition for service %s: %w", d.service, err)
	}
	return taskDefinition.EnvironmentVariables(), nil
}

// Secrets returns the secrets of the task definition.
func (d *ecsServiceDescriber) Secrets() ([]*awsecs.ContainerSecret, error) {
	taskDefinition, err := d.ecsClient.TaskDefinition(d.app, d.env, d.service)
	if err != nil {
		return nil, fmt.Errorf("describe task definition for service %s: %w", d.service, err)
	}
	return taskDefinition.Secrets(), nil
}

// Platform returns the platform of the task definition.
func (d *ecsServiceDescriber) Platform() (*awsecs.ContainerPlatform, error) {
	taskDefinition, err := d.ecsClient.TaskDefinition(d.app, d.env, d.service)
	if err != nil {
		return nil, fmt.Errorf("describe task definition for service %s: %w", d.service, err)
	}
	platform := taskDefinition.Platform()
	if platform == nil {
		return &awsecs.ContainerPlatform{
			OperatingSystem: "LINUX",
			Architecture:    "X86_64",
		}, nil
	}
	return platform, nil
}

// ServiceARN retrieves the ARN of the app runner service.
func (d *appRunnerServiceDescriber) ServiceARN() (string, error) {
	serviceStackResources, err := d.ServiceStackResources()
	if err != nil {
		return "", err
	}

	for _, resource := range serviceStackResources {
		arn := resource.PhysicalID
		if resource.Type == apprunnerServiceType && arn != "" {
			return arn, nil
		}
	}

	return "", fmt.Errorf("no App Runner Service in service stack")
}

// Service retrieves an app runner service.
func (d *appRunnerServiceDescriber) Service() (*apprunner.Service, error) {
	serviceARN, err := d.ServiceARN()
	if err != nil {
		return nil, err
	}

	service, err := d.apprunnerClient.DescribeService(serviceARN)
	if err != nil {
		return nil, fmt.Errorf("describe service: %w", err)
	}
	return service, nil
}

// ServiceURL retrieves the app runner service URL.
func (d *appRunnerServiceDescriber) ServiceURL() (string, error) {
	service, err := d.Service()
	if err != nil {
		return "", err
	}

	return formatAppRunnerUrl(service.ServiceURL), nil
}

func formatAppRunnerUrl(serviceURL string) string {
	svcUrl := &url.URL{
		Host: serviceURL,
		// App Runner defaults to https
		Scheme: "https",
	}

	return svcUrl.String()
}

// ServiceConfig contains serialized configuration parameters for a service.
type ServiceConfig struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	Platform    string `json:"platform,omitempty"`
}

type ECSServiceConfig struct {
	*ServiceConfig

	Tasks string `json:"tasks"`
}

type appRunnerConfigurations []*ServiceConfig

type ecsConfigurations []*ECSServiceConfig

func (c ecsConfigurations) humanString(w io.Writer) {
	headers := []string{"Environment", "Tasks", "CPU (vCPU)", "Memory (MiB)", "Platform", "Port"}
	var rows [][]string
	for _, config := range c {
		rows = append(rows, []string{config.Environment, config.Tasks, cpuToString(config.CPU), config.Memory, config.Platform, config.Port})
	}

	printTable(w, headers, rows)
}

func (c appRunnerConfigurations) humanString(w io.Writer) {
	headers := []string{"Environment", "CPU (vCPU)", "Memory (MiB)", "Port"}
	var rows [][]string
	for _, config := range c {
		rows = append(rows, []string{config.Environment, cpuToString(config.CPU), config.Memory, config.Port})
	}

	printTable(w, headers, rows)
}

// envVar contains serialized environment variables for a service.
type envVar struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

type envVars []*envVar

func (e envVars) humanString(w io.Writer) {
	headers := []string{"Name", "Environment", "Value"}
	var rows [][]string
	sort.SliceStable(e, func(i, j int) bool { return e[i].Environment < e[j].Environment })
	sort.SliceStable(e, func(i, j int) bool { return e[i].Name < e[j].Name })

	for _, v := range e {
		rows = append(rows, []string{v.Name, v.Environment, v.Value})
	}

	printTable(w, headers, rows)
}

type containerEnvVar struct {
	*envVar

	Container string `json:"container"`
}

type containerEnvVars []*containerEnvVar

func (e containerEnvVars) humanString(w io.Writer) {
	headers := []string{"Name", "Container", "Environment", "Value"}
	var rows [][]string
	sort.SliceStable(e, func(i, j int) bool { return e[i].Environment < e[j].Environment })
	sort.SliceStable(e, func(i, j int) bool { return e[i].Container < e[j].Container })
	sort.SliceStable(e, func(i, j int) bool { return e[i].Name < e[j].Name })

	for _, v := range e {
		rows = append(rows, []string{v.Name, v.Container, v.Environment, v.Value})
	}

	printTable(w, headers, rows)
}
