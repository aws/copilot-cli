// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// BackendServiceDescriber retrieves information about a backend service.
type BackendServiceDescriber struct {
	service         *config.Service
	enableResources bool

	store                storeSvc
	svcDescriber         svcDescriber
	initServiceDescriber func(string) error
}

// NewBackendServiceDescriber instantiates a backend service describer.
func NewBackendServiceDescriber(app, svc string) (*BackendServiceDescriber, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := store.GetService(app, svc)
	if err != nil {
		return nil, err
	}
	opts := &BackendServiceDescriber{
		service:         meta,
		enableResources: false,
		store:           store,
	}
	opts.initServiceDescriber = func(env string) error {
		d, err := NewServiceDescriber(app, env, svc)
		if err != nil {
			return err
		}
		opts.svcDescriber = d
		return nil
	}
	return opts, nil
}

// NewBackendServiceDescriberWithResources instantiates a backend service describer with stack resources.
func NewBackendServiceDescriberWithResources(app, svc string) (*BackendServiceDescriber, error) {
	d, err := NewBackendServiceDescriber(app, svc)
	if err != nil {
		return nil, err
	}
	d.enableResources = true
	return d, nil
}

// URI returns the service discovery namespace and is used to make
// BackendServiceDescriber have the same signature as WebServiceDescriber.
func (d *BackendServiceDescriber) URI(envName string) (string, error) {
	if err := d.initServiceDescriber(envName); err != nil {
		return "", err
	}
	svcParams, err := d.svcDescriber.Params()
	if err != nil {
		return "", fmt.Errorf("retrieve service deployment configuration: %w", err)
	}
	s := serviceDiscovery{
		Service: d.service.Name,
		Port:    svcParams[stack.LBWebServiceContainerPortParamKey],
		App:     d.service.App,
	}
	return s.String(), nil
}

// Describe returns info of a backend service.
func (d *BackendServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironments(d.service.App)
	if err != nil {
		return nil, fmt.Errorf("list environments for application %s: %w", d.service.App, err)
	}

	var configs []*ServiceConfig
	var services []*ServiceDiscovery
	var envVars []*EnvVars
	for _, env := range environments {
		err := d.initServiceDescriber(env.Name)
		if err != nil {
			return nil, err
		}
		svcParams, err := d.svcDescriber.Params()
		if err != nil && !IsStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve service deployment configuration: %w", err)
		}
		if err != nil {
			continue
		}
		services = appendServiceDiscovery(services, serviceDiscovery{
			Service: d.service.Name,
			Port:    svcParams[stack.LBWebServiceContainerPortParamKey],
			App:     d.service.App,
		}, env.Name)
		configs = append(configs, &ServiceConfig{
			Environment: env.Name,
			Port:        svcParams[stack.LBWebServiceContainerPortParamKey],
			Tasks:       svcParams[stack.ServiceTaskCountParamKey],
			CPU:         svcParams[stack.ServiceTaskCPUParamKey],
			Memory:      svcParams[stack.ServiceTaskMemoryParamKey],
		})
		backendSvcEnvVars, err := d.svcDescriber.EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		envVars = append(envVars, flattenEnvVars(env.Name, backendSvcEnvVars)...)
	}
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Environment < envVars[j].Environment })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	resources := make(map[string][]*CfnResource)
	if d.enableResources {
		for _, env := range environments {
			stackResources, err := d.svcDescriber.ServiceStackResources()
			if err != nil && !IsStackNotExistsErr(err) {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			if err != nil {
				continue
			}
			resources[env.Name] = flattenResources(stackResources)
		}
	}

	return &backendSvcDesc{
		Service:          d.service.Name,
		Type:             d.service.Type,
		App:              d.service.App,
		Configurations:   configs,
		ServiceDiscovery: services,
		Variables:        envVars,
		Resources:        resources,
	}, nil
}

// backendSvcDesc contains serialized parameters for a backend service.
type backendSvcDesc struct {
	Service          string             `json:"service"`
	Type             string             `json:"type"`
	App              string             `json:"application"`
	Configurations   configurations     `json:"configurations"`
	ServiceDiscovery serviceDiscoveries `json:"serviceDiscovery"`
	Variables        envVars            `json:"variables"`
	Resources        cfnResources       `json:"resources,omitempty"`
}

// JSONString returns the stringified backendService struct with json format.
func (w *backendSvcDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal backend service description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified backendService struct with human readable format.
func (w *backendSvcDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
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
		w.Resources.humanStringByEnv(writer, w.Configurations)
	}
	writer.Flush()
	return b.String()
}
