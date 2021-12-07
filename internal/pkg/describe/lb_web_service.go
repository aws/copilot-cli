// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

const (
	envOutputPublicLoadBalancerDNSName = "PublicLoadBalancerDNSName"
	envOutputSubdomain                 = "EnvironmentSubdomain"
)

type envDescriber interface {
	ServiceDiscoveryEndpoint() (string, error)
	Params() (map[string]string, error)
	Outputs() (map[string]string, error)
}

// LBWebServiceDescriber retrieves information about a load balanced web service.
type LBWebServiceDescriber struct {
	*baseServiceDescription
	svcStackDescriber map[string]ecsStackDescriber
	envDescriber      map[string]envDescriber

	// cache only last svc paramerters
	svcParams map[string]string
}

// NewLBWebServiceDescriber instantiates a load balanced service describer.
func NewLBWebServiceDescriber(opt NewServiceConfig) (*LBWebServiceDescriber, error) {
	describer := &LBWebServiceDescriber{
		baseServiceDescription: &baseServiceDescription{
			app:             opt.App,
			svc:             opt.Svc,
			enableResources: opt.EnableResources,
			store:           opt.DeployStore,
		},
		svcStackDescriber: make(map[string]ecsStackDescriber),
		envDescriber:      make(map[string]envDescriber),
	}
	describer.initDescribers = func(env string) error {
		if _, ok := describer.svcStackDescriber[env]; ok {
			return nil
		}
		svcDescr, err := NewECSServiceDescriber(NewServiceConfig{
			App:         opt.App,
			Env:         env,
			Svc:         opt.Svc,
			ConfigStore: opt.ConfigStore,
		})
		if err != nil {
			return err
		}
		describer.svcStackDescriber[env] = svcDescr
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

// Describe returns info of a web service.
func (d *LBWebServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironmentsDeployedTo(d.app, d.svc)
	if err != nil {
		return nil, fmt.Errorf("list deployed environments for application %s: %w", d.app, err)
	}

	var routes []*WebServiceRoute
	var configs []*ECSServiceConfig
	var serviceDiscoveries []*ServiceDiscovery
	var envVars []*containerEnvVar
	var secrets []*secret
	for _, env := range environments {
		err := d.initDescribers(env)
		if err != nil {
			return nil, err
		}
		webServiceURI, err := d.URI(env)
		if err != nil {
			return nil, fmt.Errorf("retrieve service URI: %w", err)
		}
		routes = append(routes, &WebServiceRoute{
			Environment: env,
			URL:         webServiceURI,
		})
		containerPlatform, err := d.svcStackDescriber[env].Platform()
		if err != nil {
			return nil, fmt.Errorf("retrieve platform: %w", err)
		}
		webSvcEnvVars, err := d.svcStackDescriber[env].EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		configs = append(configs, &ECSServiceConfig{
			ServiceConfig: &ServiceConfig{
				Environment: env,
				Port:        d.svcParams[cfnstack.LBWebServiceContainerPortParamKey],
				CPU:         d.svcParams[cfnstack.WorkloadTaskCPUParamKey],
				Memory:      d.svcParams[cfnstack.WorkloadTaskMemoryParamKey],
				Platform:    dockerengine.PlatformString(containerPlatform.OperatingSystem, containerPlatform.Architecture),
			},
			Tasks: d.svcParams[cfnstack.WorkloadTaskCountParamKey],
		})
		endpoint, err := d.envDescriber[env].ServiceDiscoveryEndpoint()
		if err != nil {
			return nil, err
		}
		serviceDiscoveries = appendServiceDiscovery(serviceDiscoveries, serviceDiscovery{
			Service:  d.svc,
			Port:     d.svcParams[cfnstack.LBWebServiceContainerPortParamKey],
			Endpoint: endpoint,
		}, env)
		envVars = append(envVars, flattenContainerEnvVars(env, webSvcEnvVars)...)
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

	return &webSvcDesc{
		Service:          d.svc,
		Type:             manifest.LoadBalancedWebServiceType,
		App:              d.app,
		Configurations:   configs,
		Routes:           routes,
		ServiceDiscovery: serviceDiscoveries,
		Variables:        envVars,
		Secrets:          secrets,
		Resources:        resources,

		environments: environments,
	}, nil
}

type secret struct {
	Name        string `json:"name"`
	Container   string `json:"container"`
	Environment string `json:"environment"`
	ValueFrom   string `json:"valueFrom"`
}

type secrets []*secret

func (s secrets) humanString(w io.Writer) {
	headers := []string{"Name", "Container", "Environment", "Value From"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	sort.SliceStable(s, func(i, j int) bool { return s[i].Environment < s[j].Environment })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Container < s[j].Container })
	sort.SliceStable(s, func(i, j int) bool { return s[i].Name < s[j].Name })
	if len(s) > 0 {
		valueFrom := s[0].ValueFrom
		if _, err := arn.Parse(s[0].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[0].ValueFrom)
		}
		fmt.Fprintf(w, "  %s\n", strings.Join([]string{s[0].Name, s[0].Container, s[0].Environment, valueFrom}, "\t"))
	}
	for prev, cur := 0, 1; cur < len(s); prev, cur = prev+1, cur+1 {
		valueFrom := s[cur].ValueFrom
		if _, err := arn.Parse(s[cur].ValueFrom); err != nil {
			// If the valueFrom is not an ARN, preface it with "parameter/"
			valueFrom = fmt.Sprintf("parameter/%s", s[cur].ValueFrom)
		}
		cols := []string{s[cur].Name, s[cur].Container, s[cur].Environment, valueFrom}
		if s[prev].Name == s[cur].Name {
			cols[0] = dittoSymbol
		}
		if s[prev].Container == s[cur].Container {
			cols[1] = dittoSymbol
		}
		if s[prev].Environment == s[cur].Environment {
			cols[2] = dittoSymbol
		}
		if s[prev].ValueFrom == s[cur].ValueFrom {
			cols[3] = dittoSymbol
		}
		fmt.Fprintf(w, "  %s\n", strings.Join(cols, "\t"))
	}
}

func underline(headings []string) []string {
	var lines []string
	for _, heading := range headings {
		line := strings.Repeat("-", len(heading))
		lines = append(lines, line)
	}
	return lines
}

// WebServiceRoute contains serialized route parameters for a web service.
type WebServiceRoute struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
}

// ServiceDiscovery contains serialized service discovery info for an service.
type ServiceDiscovery struct {
	Environment []string `json:"environment"`
	Namespace   string   `json:"namespace"`
}

type serviceDiscoveries []*ServiceDiscovery

func (s serviceDiscoveries) humanString(w io.Writer) {
	headers := []string{"Environment", "Namespace"}
	fmt.Fprintf(w, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(w, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, sd := range s {
		fmt.Fprintf(w, "  %s\t%s\n", strings.Join(sd.Environment, ", "), sd.Namespace)
	}
}

// webSvcDesc contains serialized parameters for a web service.
type webSvcDesc struct {
	Service          string               `json:"service"`
	Type             string               `json:"type"`
	App              string               `json:"application"`
	Configurations   ecsConfigurations    `json:"configurations"`
	Routes           []*WebServiceRoute   `json:"routes"`
	ServiceDiscovery serviceDiscoveries   `json:"serviceDiscovery"`
	Variables        containerEnvVars     `json:"variables"`
	Secrets          secrets              `json:"secrets,omitempty"`
	Resources        deployedSvcResources `json:"resources,omitempty"`

	environments []string
}

// JSONString returns the stringified webSvcDesc struct in json format.
func (w *webSvcDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal web service description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified webService struct in human readable format.
func (w *webSvcDesc) HumanString() string {
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
	fmt.Fprint(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	headers := []string{"Environment", "URL"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
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

func cpuToString(s string) string {
	cpuInt, _ := strconv.Atoi(s)
	cpuFloat := float64(cpuInt) / 1024
	return fmt.Sprintf("%g", cpuFloat)
}

// IsStackNotExistsErr returns true if error type is stack not exist.
func IsStackNotExistsErr(err error) bool {
	if err == nil {
		return false
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	if aerr.Code() != "ValidationError" {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	if !strings.Contains(aerr.Message(), "does not exist") {
		return IsStackNotExistsErr(errors.Unwrap(err))
	}
	return true
}

func appendServiceDiscovery(sds []*ServiceDiscovery, sd serviceDiscovery, env string) []*ServiceDiscovery {
	exist := false
	for _, s := range sds {
		if s.Namespace == sd.String() {
			s.Environment = append(s.Environment, env)
			exist = true
		}
	}
	if !exist {
		sds = append(sds, &ServiceDiscovery{
			Environment: []string{env},
			Namespace:   sd.String(),
		})
	}
	return sds
}
