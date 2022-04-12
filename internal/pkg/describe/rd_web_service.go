// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// RDWebServiceDescriber retrieves information about a request-driven web service.
type RDWebServiceDescriber struct {
	app             string
	svc             string
	enableResources bool

	store            DeployedEnvServicesLister
	initClients      func(string) error
	envSvcDescribers map[string]apprunnerDescriber
}

// NewRDWebServiceDescriber instantiates a request-driven service describer.
func NewRDWebServiceDescriber(opt NewServiceConfig) (*RDWebServiceDescriber, error) {
	describer := &RDWebServiceDescriber{
		app:             opt.App,
		svc:             opt.Svc,
		enableResources: opt.EnableResources,
		store:           opt.DeployStore,

		envSvcDescribers: make(map[string]apprunnerDescriber),
	}
	describer.initClients = func(env string) error {
		if _, ok := describer.envSvcDescribers[env]; ok {
			return nil
		}
		d, err := NewAppRunnerServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.envSvcDescribers[env] = d
		return nil
	}
	return describer, nil
}

// Describe returns info for a request-driven web service.
func (d *RDWebServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var observabilities []Observability
	var routes []*WebServiceRoute
	var configs []*ServiceConfig
	var envVars envVars
	resources := make(map[string][]*stack.Resource)
	for _, env := range environments {
		err := d.initClients(env)
		if err != nil {
			return nil, err
		}
		service, err := d.envSvcDescribers[env].Service()
		if err != nil {
			return nil, fmt.Errorf("retrieve service configuration: %w", err)
		}
		webServiceURI := formatAppRunnerUrl(service.ServiceURL)
		routes = append(routes, &WebServiceRoute{
			Environment: env,
			URL:         webServiceURI,
		})
		configs = append(configs, &ServiceConfig{
			Environment: env,
			Port:        service.Port,
			CPU:         service.CPU,
			Memory:      service.Memory,
		})
		for _, v := range service.EnvironmentVariables {
			envVars = append(envVars, &envVar{
				Environment: env,
				Name:        v.Name,
				Value:       v.Value,
			})
		}
		observabilities = append(observabilities, Observability{
			Environment: env,
			Tracing:     formatTracingConfiguration(service.Observability.TraceConfiguration),
		})
		if d.enableResources {
			stackResources, err := d.envSvcDescribers[env].ServiceStackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	isObservabilitiesEmpty := true
	for _, oc := range observabilities {
		if !oc.isEmpty() {
			isObservabilitiesEmpty = false
		}
	}
	if isObservabilitiesEmpty {
		observabilities = nil
	}

	return &rdWebSvcDesc{
		Service:                 d.svc,
		Type:                    manifest.RequestDrivenWebServiceType,
		App:                     d.app,
		AppRunnerConfigurations: configs,
		Routes:                  routes,
		Variables:               envVars,
		Resources:               resources,
		Observability:           observabilities,

		environments: environments,
	}, nil
}

func formatTracingConfiguration(configuration *apprunner.TraceConfiguration) *Tracing {
	if configuration == nil {
		return nil
	}
	return &Tracing{
		Vendor: aws.StringValue(configuration.Vendor),
	}
}

type Observability struct {
	Environment string   `json:"environment"`
	Tracing     *Tracing `json:"tracing,omitempty"`
}

type Tracing struct {
	Vendor string `json:"vendor"`
}

func (o *Observability) isEmpty() bool {
	return o.Tracing.isEmpty()
}

func (t *Tracing) isEmpty() bool {
	return t.Vendor == ""
}

// rdWebSvcDesc contains serialized parameters for a web service.
type rdWebSvcDesc struct {
	Service                 string                  `json:"service"`
	Type                    string                  `json:"type"`
	App                     string                  `json:"application"`
	AppRunnerConfigurations appRunnerConfigurations `json:"configurations"`
	Routes                  []*WebServiceRoute      `json:"routes"`
	Variables               envVars                 `json:"variables"`
	Resources               deployedSvcResources    `json:"resources,omitempty"`
	Observability           []Observability         `json:"observability,omitempty"`

	environments []string `json:"-"`
}

// JSONString returns the stringified rdWebSvcDesc struct in json format.
func (w *rdWebSvcDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal web service description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified webService struct in human readable format.
func (w *rdWebSvcDesc) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprint(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.AppRunnerConfigurations.humanString(writer)
	if w.hasObservabilityConfiguration() {
		fmt.Fprint(writer, color.Bold.Sprint("\nObservability\n\n"))
		writer.Flush()
		headers := []string{"Environment", "Tracing"}
		fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
		fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
		for _, config := range w.Observability {
			tracingVendor := "None"
			if config.Tracing != nil && config.Tracing.Vendor != "" {
				tracingVendor = config.Tracing.Vendor
			}
			fmt.Fprintf(writer, "  %s\t%s\n", config.Environment, tracingVendor)
		}
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	headers := []string{"Environment", "URL"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
	}

	fmt.Fprint(writer, color.Bold.Sprint("\nVariables\n\n"))
	writer.Flush()
	w.Variables.humanString(writer)

	if len(w.Resources) != 0 {
		fmt.Fprint(writer, color.Bold.Sprint("\nResources\n"))
		writer.Flush()

		w.Resources.humanStringByEnv(writer, w.environments)
	}
	writer.Flush()
	return b.String()
}

func (w *rdWebSvcDesc) hasObservabilityConfiguration() bool {
	for _, envObservabilityConfig := range w.Observability {
		if !envObservabilityConfig.isEmpty() {
			return true
		}
	}
	return false
}
