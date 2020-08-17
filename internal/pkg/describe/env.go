// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// EnvDescription contains the information about an environment.
type EnvDescription struct {
	Environment *config.Environment `json:"environment"`
	Services    []*config.Service   `json:"services"`
	Tags        map[string]string   `json:"tags,omitempty"`
	Resources   []*CfnResource      `json:"resources,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	app             string
	env             *config.Environment
	enableResources bool

	configStore    ConfigStoreSvc
	deployStore    DeployedEnvServicesLister
	stackDescriber stackAndResourcesDescriber
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
	sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assume role for environment %s: %w", env.ManagerRoleARN, err)
	}
	d := newStackDescriber(sess)
	return &EnvDescriber{
		app:             opt.App,
		env:             env,
		enableResources: opt.EnableResources,

		configStore:    opt.ConfigStore,
		deployStore:    opt.DeployStore,
		stackDescriber: stackAndResourcesDescriber(d),
	}, nil
}

// Describe returns info about an application's environment.
func (e *EnvDescriber) Describe() (*EnvDescription, error) {
	svcs, err := e.filterDeployedSvcs()
	if err != nil {
		return nil, err
	}

	tags, err := e.stackTags()
	if err != nil {
		return nil, fmt.Errorf("retrieve environment tags: %w", err)
	}

	var stackResources []*CfnResource
	if e.enableResources {
		stackResources, err = e.envOutputs()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment resources: %w", err)
		}
	}

	return &EnvDescription{
		Environment: e.env,
		Services:    svcs,
		Tags:        tags,
		Resources:   stackResources,
	}, nil
}

func (e *EnvDescriber) stackTags() (map[string]string, error) {
	tags := make(map[string]string)
	envStack, err := e.stackDescriber.Stack(stack.NameForEnv(e.app, e.env.Name))
	if err != nil {
		return nil, err
	}
	for _, tag := range envStack.Tags {
		tags[*tag.Key] = *tag.Value
	}
	return tags, nil
}

func (e *EnvDescriber) filterDeployedSvcs() ([]*config.Service, error) {
	allSvcs, err := e.configStore.ListServices(e.app)
	if err != nil {
		return nil, fmt.Errorf("list services for app %s: %w", e.app, err)
	}
	svcs := make(map[string]*config.Service)
	for _, svc := range allSvcs {
		svcs[svc.Name] = svc
	}
	deployedSvcNames, err := e.deployStore.ListDeployedServices(e.app, e.env.Name)
	if err != nil {
		return nil, fmt.Errorf("list deployed services in env %s: %w", e.env.Name, err)
	}
	var deployedSvcs []*config.Service
	for _, deployedSvcName := range deployedSvcNames {
		deployedSvcs = append(deployedSvcs, svcs[deployedSvcName])
	}
	return deployedSvcs, nil
}

func (e *EnvDescriber) envOutputs() ([]*CfnResource, error) {
	envStack, err := e.stackDescriber.StackResources(stack.NameForEnv(e.app, e.env.Name))
	if err != nil {
		return nil, err
	}
	outputs := flattenResources(envStack)
	return outputs, nil
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
	fmt.Fprintf(writer, "  %s\t%t\n", "Production", e.Environment.Prod)
	fmt.Fprintf(writer, "  %s\t%s\n", "Region", e.Environment.Region)
	fmt.Fprintf(writer, "  %s\t%s\n", "Account ID", e.Environment.AccountID)
	fmt.Fprint(writer, color.Bold.Sprint("\nServices\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "----", "----")
	writer.Flush()
	for _, svc := range e.Services {
		fmt.Fprintf(writer, "  %s\t%s\n", svc.Name, svc.Type)
	}
	writer.Flush()
	if len(e.Tags) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nTags\n\n"))
		writer.Flush()
		fmt.Fprintf(writer, "  %s\t%s\n", "Key", "Value")
		fmt.Fprintf(writer, "  %s\t%s\n", "---", "-----")
		// sort Tags in alpha order by keys
		keys := make([]string, 0, len(e.Tags))
		for k := range e.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(writer, "  %s\t%s\n", key, e.Tags[key])
			writer.Flush()
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
