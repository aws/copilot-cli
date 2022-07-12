// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/elbv2"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

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
	app             string
	svc             string
	enableResources bool

	store                    DeployedEnvServicesLister
	initECSServiceDescribers func(string) (ecsDescriber, error)
	initEnvDescribers        func(string) (envDescriber, error)
	initLBDescriber          func(string) (lbDescriber, error)
	ecsServiceDescribers     map[string]ecsDescriber
	envStackDescriber        map[string]envDescriber
}

// NewBackendServiceDescriber instantiates a backend service describer.
func NewBackendServiceDescriber(opt NewServiceConfig) (*BackendServiceDescriber, error) {
	describer := &BackendServiceDescriber{
		app:                  opt.App,
		svc:                  opt.Svc,
		enableResources:      opt.EnableResources,
		store:                opt.DeployStore,
		ecsServiceDescribers: make(map[string]ecsDescriber),
		envStackDescriber:    make(map[string]envDescriber),
	}
	describer.initLBDescriber = func(envName string) (lbDescriber, error) {
		env, err := opt.ConfigStore.GetEnvironment(opt.App, envName)
		if err != nil {
			return nil, fmt.Errorf("get environment %s: %w", envName, err)
		}
		sess, err := sessions.ImmutableProvider().FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return nil, err
		}
		return elbv2.New(sess), nil
	}
	describer.initECSServiceDescribers = func(env string) (ecsDescriber, error) {
		if describer, ok := describer.ecsServiceDescribers[env]; ok {
			return describer, nil
		}
		svcDescr, err := newECSServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		}, env)
		if err != nil {
			return nil, err
		}
		describer.ecsServiceDescribers[env] = svcDescr
		return svcDescr, nil
	}
	describer.initEnvDescribers = func(env string) (envDescriber, error) {
		if describer, ok := describer.envStackDescriber[env]; ok {
			return describer, nil
		}
		envDescr, err := NewEnvDescriber(NewEnvDescriberConfig{
			App:         opt.App,
			Env:         env,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return nil, err
		}
		describer.envStackDescriber[env] = envDescr
		return envDescr, nil
	}
	return describer, nil
}

// Describe returns info of a backend service.
func (d *BackendServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var routes []*WebServiceRoute
	var configs []*ECSServiceConfig
	var services []*ServiceDiscovery
	var envVars []*containerEnvVar
	var secrets []*secret
	for _, env := range environments {
		svcDescr, err := d.initECSServiceDescribers(env)
		if err != nil {
			return nil, err
		}
		uri, err := d.URI(env)
		if err != nil {
			return nil, fmt.Errorf("retrieve service URI: %w", err)
		}
		if uri.AccessType == URIAccessTypeInternal {
			routes = append(routes, &WebServiceRoute{
				Environment: env,
				URL:         uri.URI,
			})
		}
		svcParams, err := svcDescr.Params()
		if err != nil {
			return nil, fmt.Errorf("get stack parameters for environment %s: %w", env, err)
		}
		envDescr, err := d.initEnvDescribers(env)
		if err != nil {
			return nil, err
		}
		port := blankContainerPort
		if svcParams[cfnstack.WorkloadContainerPortParamKey] != cfnstack.NoExposedContainerPort {
			endpoint, err := envDescr.ServiceDiscoveryEndpoint()
			if err != nil {
				return nil, err
			}
			port = svcParams[cfnstack.WorkloadContainerPortParamKey]
			services = appendServiceDiscovery(services, serviceDiscovery{
				Service:  d.svc,
				Port:     port,
				Endpoint: endpoint,
			}, env)
		}
		containerPlatform, err := svcDescr.Platform()
		if err != nil {
			return nil, fmt.Errorf("retrieve platform: %w", err)
		}
		backendSvcEnvVars, err := svcDescr.EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		configs = append(configs, &ECSServiceConfig{
			ServiceConfig: &ServiceConfig{
				Environment: env,
				Port:        port,
				CPU:         svcParams[cfnstack.WorkloadTaskCPUParamKey],
				Memory:      svcParams[cfnstack.WorkloadTaskMemoryParamKey],
				Platform:    dockerengine.PlatformString(containerPlatform.OperatingSystem, containerPlatform.Architecture),
			},
			Tasks: svcParams[cfnstack.WorkloadTaskCountParamKey],
		})
		envVars = append(envVars, flattenContainerEnvVars(env, backendSvcEnvVars)...)
		webSvcSecrets, err := svcDescr.Secrets()
		if err != nil {
			return nil, fmt.Errorf("retrieve secrets: %w", err)
		}
		secrets = append(secrets, flattenSecrets(env, webSvcSecrets)...)
	}

	resources := make(map[string][]*stack.Resource)
	if d.enableResources {
		for _, env := range environments {
			svcDescr, err := d.initECSServiceDescribers(env)
			if err != nil {
				return nil, err
			}
			stackResources, err := svcDescr.ServiceStackResources()
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
		Routes:           routes,
		ServiceDiscovery: services,
		Variables:        envVars,
		Secrets:          secrets,
		Resources:        resources,

		environments: environments,
	}, nil
}

// Manifest returns the contents of the manifest used to deploy a backend service stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *BackendServiceDescriber) Manifest(env string) ([]byte, error) {
	cfn, err := d.initECSServiceDescribers(env)
	if err != nil {
		return nil, err
	}
	return cfn.Manifest()
}

// backendSvcDesc contains serialized parameters for a backend service.
type backendSvcDesc struct {
	Service          string               `json:"service"`
	Type             string               `json:"type"`
	App              string               `json:"application"`
	Configurations   ecsConfigurations    `json:"configurations"`
	Routes           []*WebServiceRoute   `json:"routes"`
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
	if len(w.Routes) > 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
		writer.Flush()
		headers := []string{"Environment", "URL"}
		fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
		fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
		for _, route := range w.Routes {
			fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
		}
	}
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
