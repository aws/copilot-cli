// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// BackendServiceDescriber retrieves information about a backend service.
type BackendServiceDescriber struct {
	app             string
	svc             string
	enableResources bool

	store                DeployedEnvServicesLister
	svcDescriber         map[string]svcDescriber
	sdMux                sync.RWMutex
	initServiceDescriber func(string) error
}

// NewBackendServiceConfig contains fields that initiates BackendServiceDescriber struct.
type NewBackendServiceConfig struct {
	NewServiceConfig
	EnableResources bool
	DeployStore     DeployedEnvServicesLister
}

// NewBackendServiceDescriber instantiates a backend service describer.
func NewBackendServiceDescriber(opt NewBackendServiceConfig) (*BackendServiceDescriber, error) {
	describer := &BackendServiceDescriber{
		app:             opt.App,
		svc:             opt.Svc,
		enableResources: opt.EnableResources,
		store:           opt.DeployStore,
		sdMux:           sync.RWMutex{},
		svcDescriber:    make(map[string]svcDescriber),
	}
	describer.initServiceDescriber = func(env string) error {
		describer.sdMux.RLock()
		if _, ok := describer.svcDescriber[env]; ok {
			describer.sdMux.RUnlock()
			return nil
		}
		describer.sdMux.RUnlock()
		d, err := NewServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.sdMux.Lock()
		describer.svcDescriber[env] = d
		describer.sdMux.Unlock()
		return nil
	}
	return describer, nil
}

// URI returns the service discovery namespace and is used to make
// BackendServiceDescriber have the same signature as WebServiceDescriber.
func (d *BackendServiceDescriber) URI(envName string) (string, error) {
	if err := d.initServiceDescriber(envName); err != nil {
		return "", err
	}
	d.sdMux.RLock()
	svcParams, err := d.svcDescriber[envName].Params()
	d.sdMux.RUnlock()
	if err != nil {
		return "", fmt.Errorf("retrieve service deployment configuration: %w", err)
	}
	s := serviceDiscovery{
		Service: d.svc,
		Port:    svcParams[stack.LBWebServiceContainerPortParamKey],
		App:     d.app,
	}
	return s.String(), nil
}

// envSvcDesc is the service's description for a particular environment.
type envSvcDesc struct {
	env              string
	config           *ServiceConfig
	serviceDiscovery serviceDiscovery
	envVars          map[string]string
	err              error
}

func (d *BackendServiceDescriber) description(env string) envSvcDesc {
	d.sdMux.RLock()
	svcParams, err := d.svcDescriber[env].Params()
	d.sdMux.RUnlock()
	if err != nil {
		return envSvcDesc{err: fmt.Errorf("retrieve service deployment configuration: %w", err)}
	}
	d.sdMux.RLock()
	backendSvcEnvVars, err := d.svcDescriber[env].EnvVars()
	d.sdMux.RUnlock()
	if err != nil {
		return envSvcDesc{err: fmt.Errorf("retrieve environment variables: %w", err)}
	}
	return envSvcDesc{
		env: env,
		config: &ServiceConfig{
			Environment: env,
			Port:        svcParams[stack.LBWebServiceContainerPortParamKey],
			Tasks:       svcParams[stack.ServiceTaskCountParamKey],
			CPU:         svcParams[stack.ServiceTaskCPUParamKey],
			Memory:      svcParams[stack.ServiceTaskMemoryParamKey],
		},
		envVars: backendSvcEnvVars,
		serviceDiscovery: serviceDiscovery{
			Service: d.svc,
			Port:    svcParams[stack.LBWebServiceContainerPortParamKey],
			App:     d.app,
			Env:     env,
		},
	}
}

func (d *BackendServiceDescriber) svcStackResources(envs []string) (map[string][]*CfnResource, error) {
	resources := make(map[string][]*CfnResource)
	if !d.enableResources {
		return resources, nil
	}
	resourceCh := make(chan svcStackResources, len(envs))
	defer close(resourceCh)
	for _, env := range envs {
		go func(env string) {
			err := d.initServiceDescriber(env)
			if err != nil {
				resourceCh <- svcStackResources{err: err}
				return
			}
			d.sdMux.RLock()
			stackResources, err := d.svcDescriber[env].ServiceStackResources()
			d.sdMux.RUnlock()
			if err != nil {
				resourceCh <- svcStackResources{err: fmt.Errorf("retrieve service resources: %w", err)}
				return
			}
			resourceCh <- svcStackResources{env: env, resource: stackResources}
		}(env)
	}
	for range envs {
		res := <-resourceCh
		if res.err != nil {
			return nil, res.err
		}
		resources[res.env] = flattenResources(res.resource)
	}
	return resources, nil
}

// Describe returns info of a backend service.
func (d *BackendServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	infoCh := make(chan envSvcDesc, len(environments))
	defer close(infoCh)
	for _, env := range environments {
		go func(env string) {
			err := d.initServiceDescriber(env)
			if err != nil {
				infoCh <- envSvcDesc{err: err}
				return
			}
			infoCh <- d.description(env)
		}(env)
	}
	var configs []*ServiceConfig
	var serviceDiscoveries []*ServiceDiscovery
	var envVars []*EnvVars
	for range environments {
		info := <-infoCh
		if info.err != nil {
			return nil, info.err
		}
		configs = append(configs, info.config)
		serviceDiscoveries = appendServiceDiscovery(serviceDiscoveries, info.serviceDiscovery, info.env)
		envVars = append(envVars, flattenEnvVars(info.env, info.envVars)...)
	}
	sort.SliceStable(configs, func(i, j int) bool { return configs[i].Environment < configs[j].Environment })
	for _, sd := range serviceDiscoveries {
		sort.SliceStable(sd.Environment, func(i, j int) bool { return sd.Environment[i] < sd.Environment[j] })
	}
	sort.SliceStable(serviceDiscoveries, func(i, j int) bool { return serviceDiscoveries[i].Namespace < serviceDiscoveries[j].Namespace })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Environment < envVars[j].Environment })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	resources, err := d.svcStackResources(environments)
	if err != nil {
		return nil, err
	}

	return &backendSvcDesc{
		Service:          d.svc,
		Type:             manifest.BackendServiceType,
		App:              d.app,
		Configurations:   configs,
		ServiceDiscovery: serviceDiscoveries,
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
