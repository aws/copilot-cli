// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
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

const (
	apprunnerServiceType              = "AWS::AppRunner::Service"
	apprunnerVPCIngressConnectionType = "AWS::AppRunner::VpcIngressConnection"
)

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
	Service(app, env, svc string) (*awsecs.Service, error)
}

type apprunnerClient interface {
	DescribeService(svcARN string) (*apprunner.Service, error)
	PrivateURL(vicARN string) (string, error)
}

type workloadStackDescriber interface {
	Params() (map[string]string, error)
	Outputs() (map[string]string, error)
	ServiceStackResources() ([]*stack.Resource, error)
	Manifest() ([]byte, error)
}

type ecsDescriber interface {
	workloadStackDescriber
	ServiceConnectDNSNames() ([]string, error)
	Platform() (*awsecs.ContainerPlatform, error)
	EnvVars() ([]*awsecs.ContainerEnvVar, error)
	Secrets() ([]*awsecs.ContainerSecret, error)
}

type apprunnerDescriber interface {
	workloadStackDescriber

	Service() (*apprunner.Service, error)
	ServiceARN() (string, error)
	ServiceURL() (string, error)
	IsPrivate() (bool, error)
}

type ecsSvcDesc struct {
	Service          string               `json:"service"`
	Type             string               `json:"type"`
	App              string               `json:"application"`
	Configurations   ecsConfigurations    `json:"configurations"`
	Routes           []*WebServiceRoute   `json:"routes"`
	ServiceDiscovery serviceDiscoveries   `json:"serviceDiscovery"`
	ServiceConnect   serviceConnects      `json:"serviceConnect,omitempty"`
	Variables        containerEnvVars     `json:"variables"`
	Secrets          secrets              `json:"secrets,omitempty"`
	Resources        deployedSvcResources `json:"resources,omitempty"`

	environments []string `json:"-"`
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

// ServiceConnectDNSNames returns the service connect dns names of a service.
func (d *ecsServiceDescriber) ServiceConnectDNSNames() ([]string, error) {
	service, err := d.ecsClient.Service(d.app, d.env, d.service)
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", d.service, err)
	}
	return service.ServiceConnectAliases(), nil
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

// vpcIngressConnectionARN returns the ARN of the VPC Ingress Connection
// for this service. If one does not exist, it returns errVPCIngressConnectionNotFound.
func (d *appRunnerServiceDescriber) vpcIngressConnectionARN() (string, error) {
	serviceStackResources, err := d.ServiceStackResources()
	if err != nil {
		return "", err
	}

	for _, resource := range serviceStackResources {
		arn := resource.PhysicalID
		if resource.Type == apprunnerVPCIngressConnectionType && arn != "" {
			return arn, nil
		}
	}

	return "", errVPCIngressConnectionNotFound
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

// IsPrivate returns true if the service is configured as non-public.
func (d *appRunnerServiceDescriber) IsPrivate() (bool, error) {
	_, err := d.vpcIngressConnectionARN()
	if err != nil {
		if errors.Is(err, errVPCIngressConnectionNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// ServiceURL retrieves the app runner service URL.
func (d *appRunnerServiceDescriber) ServiceURL() (string, error) {
	vicARN, err := d.vpcIngressConnectionARN()
	isVICNotFound := errors.Is(err, errVPCIngressConnectionNotFound)
	if err != nil && !isVICNotFound {
		return "", err
	}

	if !isVICNotFound {
		url, err := d.apprunnerClient.PrivateURL(vicARN)
		if err != nil {
			return "", err
		}
		return formatAppRunnerURL(url), nil
	}

	service, err := d.Service()
	if err != nil {
		return "", err
	}
	return formatAppRunnerURL(service.ServiceURL), nil
}

func formatAppRunnerURL(serviceURL string) string {
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

type secret struct {
	Name        string `json:"name"`
	Container   string `json:"container"`
	Environment string `json:"environment"`
	ValueFrom   string `json:"valueFrom"`
}

type secrets []*secret

func (s secrets) humanString(w io.Writer) {
	headers := []string{"Name", "Container", "Environment", "Value From"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	sort.SliceStable(s, func(i, j int) bool { return s[i].Environment < s[j].Environment })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Container < s[j].Container })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Name < s[j].Name })
	if len(s) > 0 {
		valueFrom := s[0].ValueFrom
		if _, err := arn.Parse(s[0].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[0].ValueFrom)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join([]string{s[0].Name, s[0].Container, s[0].Environment, valueFrom}, "\t"))
	}
	for prev, cur := 0, 1; cur < len(s); prev, cur = prev+1, cur+1 {
		valueFrom := s[cur].ValueFrom
		if _, err := arn.Parse(s[cur].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[cur].ValueFrom)
		}
		cols := []string{s[cur].Name, s[cur].Container, s[cur].Environment, valueFrom}
		if s[prev].Name == s[cur].Name {
			cols[0] = dittoSymbol
		}
		if s[prev].Container == s[cur].Container {
			cols[1] = dittoSymbol
		}
		if s[prev].Environment == s[cur].Environment {
			cols[2] = dittoSymbol
		}
		if s[prev].ValueFrom == s[cur].ValueFrom {
			cols[3] = dittoSymbol
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cols, "\t"))
	}
}

func underline(headings []string) []string {
	var lines []string
	for _, heading := range headings {
		line := strings.Repeat("-", len(heading))
		lines = append(lines, line)
	}
	return lines
}

// endpointToEnvs is a mapping of endpoint to environments.
type endpointToEnvs map[string][]string

func (e *endpointToEnvs) marshalJSON() ([]byte, error) {
	type internalEndpoint struct {
		Environment []string `json:"environment"`
		Endpoint    string   `json:"endpoint"`
	}
	var internalEndpoints []internalEndpoint
	for endpoint := range *e {
		internalEndpoints = append(internalEndpoints, internalEndpoint{
			Environment: (*e)[endpoint],
			Endpoint:    endpoint,
		})
	}
	sort.Slice(internalEndpoints, func(i, j int) bool { return internalEndpoints[i].Endpoint < internalEndpoints[j].Endpoint })
	return json.Marshal(&internalEndpoints)
}

func (e endpointToEnvs) add(endpoint string, env string) {
	e[endpoint] = append(e[endpoint], env)
}

type serviceDiscoveries endpointToEnvs

// MarshalJSON overrides the default JSON marshaling logic for the serviceDiscoveries
// struct, allowing it to perform more complex marshaling behavior.
func (sds *serviceDiscoveries) MarshalJSON() ([]byte, error) {
	return (*endpointToEnvs)(sds).marshalJSON()
}

func (sds *serviceDiscoveries) collectEndpoints(descr envDescriber, svc, env, port string) error {
	endpoint, err := descr.ServiceDiscoveryEndpoint()
	if err != nil {
		return err
	}
	sd := serviceDiscovery{
		Service:  svc,
		Port:     port,
		Endpoint: endpoint,
	}
	(*endpointToEnvs)(sds).add(sd.String(), env)
	return nil
}

type serviceConnects endpointToEnvs

// MarshalJSON overrides the default JSON marshaling logic for the serviceConnects
// struct, allowing it to perform more complex marshaling behavior.
func (scs *serviceConnects) MarshalJSON() ([]byte, error) {
	return (*endpointToEnvs)(scs).marshalJSON()
}

func (scs *serviceConnects) collectEndpoints(descr ecsDescriber, env string) error {
	scDNSNames, err := descr.ServiceConnectDNSNames()
	if err != nil {
		return fmt.Errorf("retrieve service connect DNS names: %w", err)
	}
	for _, dnsName := range scDNSNames {
		(*endpointToEnvs)(scs).add(dnsName, env)
	}
	return nil
}

type serviceEndpoints struct {
	discoveries serviceDiscoveries
	connects    serviceConnects
}

func (s serviceEndpoints) humanString(w io.Writer) {
	headers := []string{"Endpoint", "Environment", "Type"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	var scEndpoints []string
	for endpoint := range s.connects {
		scEndpoints = append(scEndpoints, endpoint)
	}
	sort.Slice(scEndpoints, func(i, j int) bool { return scEndpoints[i] < scEndpoints[j] })
	for _, endpoint := range scEndpoints {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", endpoint, strings.Join(s.connects[endpoint], ", "), "Service Connect")
	}
	var sdEndpoints []string
	for endpoint := range s.discoveries {
		sdEndpoints = append(sdEndpoints, endpoint)
	}
	sort.Slice(sdEndpoints, func(i, j int) bool { return sdEndpoints[i] < sdEndpoints[j] })
	for _, endpoint := range sdEndpoints {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", endpoint, strings.Join(s.discoveries[endpoint], ", "), "Service Discovery")
	}
}
