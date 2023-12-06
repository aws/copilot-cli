// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

var (
	fmtLegacySvcDiscoveryEndpoint = "%s.local"
)

// EnvDescription contains the information about an environment.
type EnvDescription struct {
	Environment    *config.Environment `json:"environment"`
	Services       []*config.Workload  `json:"services"`
	Jobs           []*config.Workload  `json:"jobs"`
	Tags           map[string]string   `json:"tags,omitempty"`
	Resources      []*stack.Resource   `json:"resources,omitempty"`
	EnvironmentVPC EnvironmentVPC      `json:"environmentVPC"`
}

// EnvironmentVPC holds the ID of the environment's VPC configuration.
type EnvironmentVPC struct {
	ID               string   `json:"id"`
	PublicSubnetIDs  []string `json:"publicSubnetIDs"`
	PrivateSubnetIDs []string `json:"privateSubnetIDs"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	app             string
	env             *config.Environment
	enableResources bool

	configStore ConfigStoreSvc
	deployStore DeployedEnvServicesLister
	cfn         stackDescriber

	// Cached values for reuse.
	description *EnvDescription
}

// NewEnvDescriberConfig contains fields that initiates EnvDescriber struct.
type NewEnvDescriberConfig struct {
	App             string
	Env             string
	EnableResources bool
	ConfigStore     ConfigStoreSvc
	DeployStore     DeployedEnvServicesLister
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(opt NewEnvDescriberConfig) (*EnvDescriber, error) {
	env, err := opt.ConfigStore.GetEnvironment(opt.App, opt.Env)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	sess, err := sessions.ImmutableProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assume role for environment %s: %w", env.ManagerRoleARN, err)
	}
	return &EnvDescriber{
		app:             opt.App,
		env:             env,
		enableResources: opt.EnableResources,

		configStore: opt.ConfigStore,
		deployStore: opt.DeployStore,
		cfn:         stack.NewStackDescriber(cfnstack.NameForEnv(opt.App, opt.Env), sess),
	}, nil
}

// Describe returns info about an application's environment.
func (d *EnvDescriber) Describe() (*EnvDescription, error) {
	if d.description != nil {
		return d.description, nil
	}
	svcs, err := d.filterDeployedSvcs()
	if err != nil {
		return nil, err
	}

	jobs, err := d.filterDeployedJobs()
	if err != nil {
		return nil, err
	}

	tags, environmentVPC, err := d.loadStackInfo()
	if err != nil {
		return nil, err
	}

	var stackResources []*stack.Resource
	if d.enableResources {
		stackResources, err = d.cfn.Resources()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment resources: %w", err)
		}
	}
	d.description = &EnvDescription{
		Environment:    d.env,
		Services:       svcs,
		Jobs:           jobs,
		Tags:           tags,
		Resources:      stackResources,
		EnvironmentVPC: environmentVPC,
	}
	return d.description, nil
}

// Manifest returns the contents of the manifest used to deploy an environment stack.
func (d *EnvDescriber) Manifest() ([]byte, error) {
	tpl, err := d.cfn.StackMetadata()
	if err != nil {
		return nil, err
	}

	metadata := struct {
		Manifest string `yaml:"Manifest"`
	}{}
	if err := yaml.Unmarshal([]byte(tpl), &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal Metadata.Manifest in environment stack: %v", err)
	}

	if metadata.Manifest != "" {
		return []byte(strings.TrimSpace(metadata.Manifest)), nil
	}
	// Otherwise, the Manifest wasn't written into the CloudFormation template, we'll convert the config in SSM.
	mft := manifest.FromEnvConfig(d.env, template.New())
	out, err := yaml.Marshal(mft)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest generated from SSM: %v", err)
	}
	return []byte(strings.TrimSpace(string(out))), nil
}

// Params returns the parameters of the environment stack.
func (d *EnvDescriber) Params() (map[string]string, error) {
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	return descr.Parameters, nil
}

// Outputs returns the outputs of the environment stack.
func (d *EnvDescriber) Outputs() (map[string]string, error) {
	descr, err := d.cfn.Describe()
	if err != nil {
		return nil, err
	}
	return descr.Outputs, nil
}

// AvailableFeatures returns the available features of the environment stack.
func (d *EnvDescriber) AvailableFeatures() ([]string, error) {
	params, err := d.Params()
	if err != nil {
		return nil, err
	}
	var availableFeatures []string
	for _, f := range template.AvailableEnvFeatures() {
		if _, ok := params[f]; ok {
			availableFeatures = append(availableFeatures, f)
		}
	}
	return availableFeatures, nil
}

// Version returns the CloudFormation template version associated with
// the environment by reading the Metadata.Version field from the template.
//
// If the Version field does not exist, then it's a legacy template and it returns an version.LegacyEnvTemplate and nil error.
func (d *EnvDescriber) Version() (string, error) {
	return stackVersion(d.cfn, version.LegacyEnvTemplate)
}

// ServiceDiscoveryEndpoint returns the endpoint the environment was initialized with, if any. Otherwise,
// it returns the legacy app.local endpoint.
func (d *EnvDescriber) ServiceDiscoveryEndpoint() (string, error) {
	p, err := d.Params()
	if err != nil {
		return "", fmt.Errorf("get params of environment %s in app %s: %w", d.env.Name, d.env.App, err)
	}
	for k, v := range p {
		// Ignore non-svc discovery params
		if k != cfnstack.EnvParamServiceDiscoveryEndpoint {
			continue
		}
		// Stacks upgraded from legacy environments will have `app.local` as the parameter value.
		// Stacks created after 1.5.0 will use `env.app.local`.
		if v != "" {
			return v, nil
		}
	}
	// If the param does not exist, the environment is legacy, has not been upgraded, and uses `app.local`.
	return fmt.Sprintf(fmtLegacySvcDiscoveryEndpoint, d.app), nil
}

func (d *EnvDescriber) loadStackInfo() (map[string]string, EnvironmentVPC, error) {
	var environmentVPC EnvironmentVPC

	envStack, err := d.cfn.Describe()
	if err != nil {
		return nil, environmentVPC, fmt.Errorf("retrieve environment stack: %w", err)
	}

	for k, v := range envStack.Outputs {
		switch k {
		case cfnstack.EnvOutputVPCID:
			environmentVPC.ID = v
		case cfnstack.EnvOutputPublicSubnets:
			environmentVPC.PublicSubnetIDs = strings.Split(v, ",")
		case cfnstack.EnvOutputPrivateSubnets:
			environmentVPC.PrivateSubnetIDs = strings.Split(v, ",")
		}
	}

	return envStack.Tags, environmentVPC, nil
}

func (d *EnvDescriber) filterDeployedSvcs() ([]*config.Workload, error) {
	allSvcs, err := d.configStore.ListServices(d.app)
	if err != nil {
		return nil, fmt.Errorf("list services for app %s: %w", d.app, err)
	}
	svcs := make(map[string]*config.Workload)
	for _, svc := range allSvcs {
		svcs[svc.Name] = svc
	}
	deployedSvcNames, err := d.deployStore.ListDeployedServices(d.app, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("list deployed services in env %s: %w", d.env.Name, err)
	}
	var deployedSvcs []*config.Workload
	for _, deployedSvcName := range deployedSvcNames {
		deployedSvcs = append(deployedSvcs, svcs[deployedSvcName])
	}
	return deployedSvcs, nil
}

// filterDeployedJobs lists the jobs that are deployed on the given app and environment
func (d *EnvDescriber) filterDeployedJobs() ([]*config.Workload, error) {
	allJobs, err := d.configStore.ListJobs(d.app)
	if err != nil {
		return nil, fmt.Errorf("list jobs for app %s: %w", d.app, err)
	}
	jobs := make(map[string]*config.Workload)
	for _, job := range allJobs {
		jobs[job.Name] = job
	}
	deployedJobNames, err := d.deployStore.ListDeployedJobs(d.app, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("list deployed jobs in env %s: %w", d.env.Name, err)
	}
	var deployedJobs []*config.Workload
	for _, deployedJobName := range deployedJobNames {
		deployedJobs = append(deployedJobs, jobs[deployedJobName])
	}
	return deployedJobs, nil
}

// ValidateCFServiceDomainAliases returns error if an environment using cdn is deployed without specifying http.alias for all load-balanced web services
func (d *EnvDescriber) ValidateCFServiceDomainAliases() error {
	stackDescr, err := d.cfn.Describe()
	if err != nil {
		return fmt.Errorf("describe stack: %w", err)
	}

	servicesString, ok := stackDescr.Parameters[cfnstack.EnvParamALBWorkloadsKey]
	if !ok || servicesString == "" {
		return nil
	}
	services := strings.Split(servicesString, ",")

	jsonOutput, ok := stackDescr.Parameters[cfnstack.EnvParamAliasesKey]
	if !ok {
		return fmt.Errorf("cannot find %s in env stack parameter set", cfnstack.EnvParamAliasesKey)
	}

	var aliases map[string][]string
	if jsonOutput != "" {
		err = json.Unmarshal([]byte(jsonOutput), &aliases)
		if err != nil {
			return fmt.Errorf("unmarshal %q: %w", jsonOutput, err)
		}
	}

	var lbSvcsWithoutAlias []string
	for _, service := range services {
		if _, ok := aliases[service]; !ok {
			lbSvcsWithoutAlias = append(lbSvcsWithoutAlias, service)
		}
	}
	if len(lbSvcsWithoutAlias) != 0 {
		return &errLBWebSvcsOnCFWithoutAlias{
			services:   lbSvcsWithoutAlias,
			aliasField: "http.alias",
		}
	}

	return nil
}

// JSONString returns the stringified EnvDescription struct with json format.
func (e *EnvDescription) JSONString() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("marshal environment description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified EnvDescription struct with human readable format.
func (e *EnvDescription) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", e.Environment.Name)
	fmt.Fprintf(writer, "  %s\t%s\n", "Region", e.Environment.Region)
	fmt.Fprintf(writer, "  %s\t%s\n", "Account ID", e.Environment.AccountID)
	fmt.Fprint(writer, color.Bold.Sprint("\nWorkloads\n\n"))
	writer.Flush()
	headers := []string{"Name", "Type"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, svc := range e.Services {
		fmt.Fprintf(writer, "  %s\t%s\n", svc.Name, svc.Type)
	}
	for _, job := range e.Jobs {
		fmt.Fprintf(writer, "  %s\t%s\n", job.Name, job.Type)
	}
	writer.Flush()
	if len(e.Tags) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nTags\n\n"))
		writer.Flush()
		headers := []string{"Key", "Value"}
		fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
		fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
		// sort Tags in alpha order by keys
		keys := make([]string, 0, len(e.Tags))
		for k := range e.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(writer, "  %s\t%s\n", key, e.Tags[key])
		}
	}
	writer.Flush()
	if len(e.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n\n"))
		writer.Flush()
		for _, resource := range e.Resources {
			fmt.Fprintf(writer, "  %s\t%s\n", resource.Type, resource.PhysicalID)
		}
	}
	writer.Flush()
	return b.String()
}
