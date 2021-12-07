// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"text/tabwriter"

	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	// BlankServiceDiscoveryURI is an empty URI to denote services
	// that cannot be reached with Service Discovery.
	BlankServiceDiscoveryURI = "-"
	blankContainerPort       = "-"
)

// BackendServiceDescriber retrieves information about a backend service.
type BackendServiceDescriber struct {
	*baseServiceDescription
	svcStackDescriber map[string]ecsStackDescriber
	envDescriber map[string]envDescriber
}

// NewBackendServiceDescriber instantiates a backend service describer.
func NewBackendServiceDescriber(opt NewServiceConfig) (*BackendServiceDescriber, error) {
	describer := &BackendServiceDescriber{
		baseServiceDescription: &baseServiceDescription{
			app:               opt.App,
			svc:               opt.Svc,
			enableResources:   opt.EnableResources,
			store:             opt.DeployStore,
		},
		svcStackDescriber: make(map[string]ecsStackDescriber),
		envDescriber: make(map[string]envDescriber),
	}
	describer.initDescribers = func(env string) error {
		if _, ok := describer.svcStackDescriber[env]; ok {
			return nil
		}
		d, err := NewECSServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.svcStackDescriber[env] = d
		envDescr, err := NewEnvDescriber(NewEnvDescriberConfig{
			App:         opt.App,
			Env:         env,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.envDescriber[env] = envDescr
		return nil
	}
	return describer, nil
}

// Describe returns info of a backend service.
func (d *BackendServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var configs []*ECSServiceConfig
	var services []*ServiceDiscovery
	var platform string
	var envVars []*containerEnvVar
	var secrets []*secret
	for _, env := range environments {
		err := d.initDescribers(env)
		if err != nil {
			return nil, err
		}
		svcParams, err := d.svcStackDescriber[env].Params()
		if err != nil {
			return nil, fmt.Errorf("get stack parameters for environment %s: %w", env, err)
		}
		port := blankContainerPort
		if svcParams[cfnstack.LBWebServiceContainerPortParamKey] != cfnstack.NoExposedContainerPort {
			endpoint, err := d.envDescriber[env].ServiceDiscoveryEndpoint()
			if err != nil {
				return nil, err
			}
			port = svcParams[cfnstack.LBWebServiceContainerPortParamKey]
			services = appendServiceDiscovery(services, serviceDiscovery{
				Service:  d.svc,
				Port:     port,
				Endpoint: endpoint,
			}, env)
		}
		containerPlatform, err := d.svcStackDescriber[env].Platform()
		if err != nil {
			return nil, fmt.Errorf("retrieve platform: %w", err)
		}
		platform = dockerengine.PlatformString(containerPlatform.OperatingSystem, containerPlatform.Architecture)
		backendSvcEnvVars, err := d.svcStackDescriber[env].EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		configs = append(configs, &ECSServiceConfig{
			ServiceConfig: &ServiceConfig{
				Environment: env,
				Port:        port,
				CPU:         svcParams[cfnstack.WorkloadTaskCPUParamKey],
				Memory:      svcParams[cfnstack.WorkloadTaskMemoryParamKey],
				Platform:    platform,
			},
			Tasks: svcParams[cfnstack.WorkloadTaskCountParamKey],
		})
		envVars = append(envVars, flattenContainerEnvVars(env, backendSvcEnvVars)...)
		webSvcSecrets, err := d.svcStackDescriber[env].Secrets()
		if err != nil {
			return nil, fmt.Errorf("retrieve secrets: %w", err)
		}
		secrets = append(secrets, flattenSecrets(env, webSvcSecrets)...)
	}

	resources := make(map[string][]*stack.Resource)
	if d.enableResources {
		for _, env := range environments {
			err := d.initDescribers(env)
			if err != nil {
				return nil, err
			}
			stackResources, err := d.svcStackDescriber[env].ServiceStackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	return &backendSvcDesc{
		Service:          d.svc,
		Type:             manifest.BackendServiceType,
		App:              d.app,
		Configurations:   configs,
		ServiceDiscovery: services,
		Variables:        envVars,
		Secrets:          secrets,
		Resources:        resources,

		environments: environments,
	}, nil
}

// backendSvcDesc contains serialized parameters for a backend service.
type backendSvcDesc struct {
	Service          string               `json:"service"`
	Type             string               `json:"type"`
	App              string               `json:"application"`
	Configurations   ecsConfigurations    `json:"configurations"`
	ServiceDiscovery serviceDiscoveries   `json:"serviceDiscovery"`
	Variables        containerEnvVars     `json:"variables"`
	Secrets          secrets              `json:"secrets,omitempty"`
	Resources        deployedSvcResources `json:"resources,omitempty"`

	environments []string `json:"-"`
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
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprint(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.Configurations.humanString(writer)
	fmt.Fprint(writer, color.Bold.Sprint("\nService Discovery\n\n"))
	writer.Flush()
	w.ServiceDiscovery.humanString(writer)
	fmt.Fprint(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	w.Variables.humanString(writer)
	if len(w.Secrets) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nSecrets\n\n"))
		writer.Flush()
		w.Secrets.humanString(writer)
	}
	if len(w.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		w.Resources.humanStringByEnv(writer, w.environments)
	}
	writer.Flush()
	return b.String()
}
