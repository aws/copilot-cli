// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

	store                  DeployedEnvServicesLister
	initAppRunnerDescriber func(string) (apprunnerDescriber, error)
	envSvcDescribers       map[string]apprunnerDescriber
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
	describer.initAppRunnerDescriber = func(env string) (apprunnerDescriber, error) {
		if describer, ok := describer.envSvcDescribers[env]; ok {
			return describer, nil
		}
		d, err := newAppRunnerServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		}, env)
		if err != nil {
			return nil, err
		}
		describer.envSvcDescribers[env] = d
		return d, nil
	}
	return describer, nil
}

// ServiceARN retrieves the ARN of the app runner service.
func (d *RDWebServiceDescriber) ServiceARN(env string) (string, error) {
	describer, err := d.initAppRunnerDescriber(env)
	if err != nil {
		return "", err
	}
	return describer.ServiceARN()
}

// Describe returns info for a request-driven web service.
func (d *RDWebServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var observabilities []observabilityInEnv
	var routes []*WebServiceRoute
	var configs []*ServiceConfig
	var envVars envVars
	resources := make(map[string][]*stack.Resource)
	for _, env := range environments {
		describer, err := d.initAppRunnerDescriber(env)
		if err != nil {
			return nil, err
		}
		service, err := describer.Service()
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
		observabilities = append(observabilities, observabilityInEnv{
			Environment: env,
			Tracing:     formatTracingConfiguration(service.Observability.TraceConfiguration),
		})
		if d.enableResources {
			stackResources, err := describer.ServiceStackResources()
			if err != nil {
				return nil, fmt.Errorf("retrieve service resources: %w", err)
			}
			resources[env] = stackResources
		}
	}

	if !observabilityPerEnv(observabilities).hasObservabilityConfiguration() {
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

// Manifest returns the contents of the manifest used to deploy a request-driven web service stack.
// If the Manifest metadata doesn't exist in the stack template, then returns ErrManifestNotFoundInTemplate.
func (d *RDWebServiceDescriber) Manifest(env string) ([]byte, error) {
	cfn, err := d.initAppRunnerDescriber(env)
	if err != nil {
		return nil, err
	}
	return cfn.Manifest()
}

func formatTracingConfiguration(configuration *apprunner.TraceConfiguration) *tracing {
	if configuration == nil {
		return nil
	}
	return &tracing{
		Vendor: aws.StringValue(configuration.Vendor),
	}
}

type observabilityPerEnv []observabilityInEnv

func (obs observabilityPerEnv) hasObservabilityConfiguration() bool {
	for _, envObservabilityConfig := range obs {
		if !envObservabilityConfig.isEmpty() {
			return true
		}
	}
	return false
}

func (obs observabilityPerEnv) humanString(w io.Writer) {
	headers := []string{"Environment", "Tracing"}
	var rows [][]string
	for _, ob := range obs {
		tracingVendor := "None"
		if ob.Tracing != nil && ob.Tracing.Vendor != "" {
			tracingVendor = ob.Tracing.Vendor
		}
		rows = append(rows, []string{ob.Environment, tracingVendor})
	}
	printTable(w, headers, rows)
}

type observabilityInEnv struct {
	Environment string   `json:"environment"`
	Tracing     *tracing `json:"tracing,omitempty"`
}

func (o observabilityInEnv) isEmpty() bool {
	return o.Tracing.isEmpty()
}

type tracing struct {
	Vendor string `json:"vendor"`
}

func (t *tracing) isEmpty() bool {
	return t == nil || t.Vendor == ""
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
	Observability           observabilityPerEnv     `json:"observability,omitempty"`

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
	if w.Observability.hasObservabilityConfiguration() {
		fmt.Fprint(writer, color.Bold.Sprint("\nObservability\n\n"))
		writer.Flush()
		w.Observability.humanString(writer)
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
