// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

// BackendAppDescriber retrieves information about a backend application.
type BackendAppDescriber struct {
	app             *archer.Application
	enableResources bool

	store            storeSvc
	appDescriber     appDescriber
	initAppDescriber func(string) error
}

// NewBackendAppDescriber instantiates a backend application describer.
func NewBackendAppDescriber(project, app string) (*BackendAppDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetApplication(project, app)
	if err != nil {
		return nil, err
	}
	opts := &BackendAppDescriber{
		app:             meta,
		enableResources: false,
		store:           svc,
	}
	opts.initAppDescriber = func(env string) error {
		d, err := NewAppDescriber(project, env, app)
		if err != nil {
			return err
		}
		opts.appDescriber = d
		return nil
	}
	return opts, nil
}

// NewBackendAppDescriberWithResources instantiates a backend application with stack resources.
func NewBackendAppDescriberWithResources(project, app string) (*BackendAppDescriber, error) {
	d, err := NewBackendAppDescriber(project, app)
	if err != nil {
		return nil, err
	}
	d.enableResources = true
	return d, nil
}

// Describe returns info of a backend application.
func (d *BackendAppDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironments(d.app.Project)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var configs []*AppConfig
	var services []*ServiceDiscovery
	var envVars []*EnvVars
	for _, env := range environments {
		err := d.initAppDescriber(env.Name)
		if err != nil {
			return nil, err
		}
		appParams, err := d.appDescriber.Params()
		if err == nil {
			services = appendServiceDiscovery(services, serviceDiscovery{
				AppName:     d.app.Name,
				Port:        appParams[stack.LBWebAppContainerPortParamKey],
				ProjectName: d.app.Project,
			}, env.Name)
			configs = append(configs, &AppConfig{
				Environment: env.Name,
				Port:        appParams[stack.LBWebAppContainerPortParamKey],
				Tasks:       appParams[stack.AppTaskCountParamKey],
				CPU:         appParams[stack.AppTaskCPUParamKey],
				Memory:      appParams[stack.AppTaskMemoryParamKey],
			})
			backendAppEnvVars, err := d.appDescriber.EnvVars()
			if err != nil {
				return nil, fmt.Errorf("retrieve environment variables: %w", err)
			}
			envVars = append(envVars, flattenEnvVars(env.Name, backendAppEnvVars)...)
			continue
		}
		if !IsStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve application deployment configuration: %w", err)
		}
	}
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Environment < envVars[j].Environment })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	resources := make(map[string][]*CfnResource)
	if d.enableResources {
		for _, env := range environments {
			stackResources, err := d.appDescriber.AppStackResources()
			if err == nil {
				resources[env.Name] = flattenResources(stackResources)
				continue
			}
			if !IsStackNotExistsErr(err) {
				return nil, fmt.Errorf("retrieve application resources: %w", err)
			}
		}
	}

	return &BackendAppDesc{
		AppName:          d.app.Name,
		Type:             d.app.Type,
		Project:          d.app.Project,
		Configurations:   configs,
		ServiceDiscovery: services,
		Variables:        envVars,
		Resources:        resources,
	}, nil
}

// BackendAppDesc contains serialized parameters for a backend application.
type BackendAppDesc struct {
	AppName          string                    `json:"appName"`
	Type             string                    `json:"type"`
	Project          string                    `json:"project"`
	Configurations   []*AppConfig              `json:"configurations"`
	ServiceDiscovery []*ServiceDiscovery       `json:"serviceDiscovery"`
	Variables        []*EnvVars                `json:"variables"`
	Resources        map[string][]*CfnResource `json:"resources,omitempty"`
}

// JSONString returns the stringified BackendApp struct with json format.
func (w *BackendAppDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified BackendApp struct with human readable format.
func (w *BackendAppDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Project", w.Project)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.AppName)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprintf(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\n", "Environment", "Tasks", "CPU (vCPU)", "Memory (MiB)", "Port")
	for _, config := range w.Configurations {
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\n", config.Environment, config.Tasks, cpuToString(config.CPU), config.Memory, config.Port)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nService Discovery\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Environment", "Namespace")
	for _, sd := range w.ServiceDiscovery {
		fmt.Fprintf(writer, "  %s\t%s\n", strings.Join(sd.Environment, ", "), sd.Namespace)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Name", "Environment", "Value")
	var prevName string
	var prevValue string
	for _, variable := range w.Variables {
		// Instead of re-writing the same variable value, we replace it with "-" to reduce text.
		if variable.Name != prevName {
			if variable.Value != prevValue {
				fmt.Fprintf(writer, "  %s\t%s\t%s\n", variable.Name, variable.Environment, variable.Value)
			} else {
				fmt.Fprintf(writer, "  %s\t%s\t-\n", variable.Name, variable.Environment)
			}
		} else {
			if variable.Value != prevValue {
				fmt.Fprintf(writer, "  -\t%s\t%s\n", variable.Environment, variable.Value)
			} else {
				fmt.Fprintf(writer, "  -\t%s\t-\n", variable.Environment)
			}
		}
		prevName = variable.Name
		prevValue = variable.Value
	}
	if len(w.Resources) != 0 {
		fmt.Fprintf(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		// Go maps don't have a guaranteed order.
		// Show the resources by the order of environments displayed under Routes for a consistent view.
		for _, config := range w.Configurations {
			env := config.Environment
			resources := w.Resources[env]
			fmt.Fprintf(writer, "\n  %s\n", env)
			for _, resource := range resources {
				fmt.Fprintf(writer, "    %s\t%s\n", resource.Type, resource.PhysicalID)
			}
		}
	}
	writer.Flush()
	return b.String()
}
