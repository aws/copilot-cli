// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
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
	meta, err := svc.GetService(project, app)
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

// URI returns the service discovery namespace and is used to make
// BackendAppDescriber have the same signature as WebAppDescriber.
func (d *BackendAppDescriber) URI(envName string) (string, error) {
	if err := d.initAppDescriber(envName); err != nil {
		return "", err
	}
	appParams, err := d.appDescriber.Params()
	if err != nil {
		return "", fmt.Errorf("retrieve application deployment configuration: %w", err)
	}
	s := serviceDiscovery{
		AppName:     d.app.Name,
		Port:        appParams[stack.LBWebAppContainerPortParamKey],
		ProjectName: d.app.Project,
	}
	return s.String(), nil
}

// Describe returns info of a backend application.
func (d *BackendAppDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironments(d.app.Project)
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", d.app.Project, err)
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
		if err != nil && !IsStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve application deployment configuration: %w", err)
		}
		if err != nil {
			continue
		}
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
	}
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Environment < envVars[j].Environment })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	resources := make(map[string][]*CfnResource)
	if d.enableResources {
		for _, env := range environments {
			stackResources, err := d.appDescriber.AppStackResources()
			if err != nil && !IsStackNotExistsErr(err) {
				return nil, fmt.Errorf("retrieve application resources: %w", err)
			}
			if err != nil {
				continue
			}
			resources[env.Name] = flattenResources(stackResources)
		}
	}

	return &backendAppDesc{
		AppName:          d.app.Name,
		Type:             d.app.Type,
		Project:          d.app.Project,
		Configurations:   configs,
		ServiceDiscovery: services,
		Variables:        envVars,
		Resources:        resources,
	}, nil
}

// backendAppDesc contains serialized parameters for a backend application.
type backendAppDesc struct {
	AppName          string             `json:"appName"`
	Type             string             `json:"type"`
	Project          string             `json:"project"`
	Configurations   configurations     `json:"configurations"`
	ServiceDiscovery serviceDiscoveries `json:"serviceDiscovery"`
	Variables        envVars            `json:"variables"`
	Resources        cfnResources       `json:"resources,omitempty"`
}

// JSONString returns the stringified BackendApp struct with json format.
func (w *backendAppDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified BackendApp struct with human readable format.
func (w *backendAppDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Project", w.Project)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.AppName)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprintf(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.Configurations.humanString(writer)
	fmt.Fprintf(writer, color.Bold.Sprint("\nService Discovery\n\n"))
	writer.Flush()
	w.ServiceDiscovery.humanString(writer)
	fmt.Fprintf(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	w.Variables.humanString(writer)
	if len(w.Resources) != 0 {
		fmt.Fprintf(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		// Go maps don't have a guaranteed order.
		// Show the resources by the order of environments displayed under Configurations for a consistent view.
		w.Resources.humanString(writer, w.Configurations)
	}
	writer.Flush()
	return b.String()
}
