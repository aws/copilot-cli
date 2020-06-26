// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// WebServiceURI represents the unique identifier to access a web service.
type WebServiceURI struct {
	DNSName string // The environment's subdomain if the service is served on HTTPS. Otherwise, the public load balancer's DNS.
	Path    string // Empty if the service is served on HTTPS. Otherwise, the pattern used to match the service.
}

func (uri *WebServiceURI) String() string {
	switch uri.Path {
	// When the service is using host based routing, the service
	// is included in the DNS name (svc.myenv.myproj.dns.com)
	case "":
		return fmt.Sprintf("https://%s", uri.DNSName)
	// When the service is using the root path, there is no "path"
	// (for example http://lb.us-west-2.amazon.com/)
	case "/":
		return fmt.Sprintf("http://%s", uri.DNSName)
	// Otherwise, if there is a path for the service, link to the
	// LoadBalancer DNS name and the path
	// (for example http://lb.us-west-2.amazon.com/svc)
	default:
		return fmt.Sprintf("http://%s/%s", uri.DNSName, uri.Path)
	}
}

type serviceDiscovery struct {
	Service string
	App     string
	Port    string
}

func (s *serviceDiscovery) String() string {
	return fmt.Sprintf("%s.%s.local:%s", s.Service, s.App, s.Port)
}

type storeSvc interface {
	GetEnvironment(appName string, environmentName string) (*config.Environment, error)
	ListEnvironments(appName string) ([]*config.Environment, error)
}

type svcDescriber interface {
	Params() (map[string]string, error)
	EnvOutputs() (map[string]string, error)
	EnvVars() (map[string]string, error)
	GetServiceArn() (*ecs.ServiceArn, error)
	ServiceStackResources() ([]*cloudformation.StackResource, error)
}

// WebServiceDescriber retrieves information about a load balanced web service.
type WebServiceDescriber struct {
	service         *config.Service
	enableResources bool

	store                storeSvc
	svcDescriber         svcDescriber
	initServiceDescriber func(string) error
}

// NewWebServiceDescriber instantiates a load balanced service describer.
func NewWebServiceDescriber(app, svc string) (*WebServiceDescriber, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := store.GetService(app, svc)
	if err != nil {
		return nil, err
	}
	opts := &WebServiceDescriber{
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

// NewWebServiceDescriberWithResources instantiates a load balanced service with stack resources.
func NewWebServiceDescriberWithResources(app, svc string) (*WebServiceDescriber, error) {
	d, err := NewWebServiceDescriber(app, svc)
	if err != nil {
		return nil, err
	}
	d.enableResources = true
	return d, nil
}

// Describe returns info of a web service.
func (d *WebServiceDescriber) Describe() (HumanJSONStringer, error) {
	environments, err := d.store.ListEnvironments(d.service.App)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var routes []*WebServiceRoute
	var configs []*ServiceConfig
	var serviceDiscoveries []*ServiceDiscovery
	var envVars []*EnvVars
	for _, env := range environments {
		err := d.initServiceDescriber(env.Name)
		if err != nil {
			return nil, err
		}
		webServiceURI, err := d.URI(env.Name)
		if err != nil && !IsStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve service URI: %w", err)
		}
		if err != nil {
			continue
		}
		routes = append(routes, &WebServiceRoute{
			Environment: env.Name,
			URL:         webServiceURI,
		})
		svcParams, err := d.svcDescriber.Params()
		if err != nil {
			return nil, fmt.Errorf("retrieve service deployment configuration: %w", err)
		}
		configs = append(configs, &ServiceConfig{
			Environment: env.Name,
			Port:        svcParams[stack.LBWebServiceContainerPortParamKey],
			Tasks:       svcParams[stack.ServiceTaskCountParamKey],
			CPU:         svcParams[stack.ServiceTaskCPUParamKey],
			Memory:      svcParams[stack.ServiceTaskMemoryParamKey],
		})
		serviceDiscoveries = appendServiceDiscovery(serviceDiscoveries, serviceDiscovery{
			Service: d.service.Name,
			Port:    svcParams[stack.LBWebServiceContainerPortParamKey],
			App:     d.service.App,
		}, env.Name)
		webSvcEnvVars, err := d.svcDescriber.EnvVars()
		if err != nil {
			return nil, fmt.Errorf("retrieve environment variables: %w", err)
		}
		envVars = append(envVars, flattenEnvVars(env.Name, webSvcEnvVars)...)
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

	return &webSvcDesc{
		Service:          d.service.Name,
		Type:             d.service.Type,
		App:              d.service.App,
		Configurations:   configs,
		Routes:           routes,
		ServiceDiscovery: serviceDiscoveries,
		Variables:        envVars,
		Resources:        resources,
	}, nil
}

// URI returns the WebServiceURI to identify this service uniquely given an environment name.
func (d *WebServiceDescriber) URI(envName string) (string, error) {
	err := d.initServiceDescriber(envName)
	if err != nil {
		return "", err
	}

	envOutputs, err := d.svcDescriber.EnvOutputs()
	if err != nil {
		return "", fmt.Errorf("get output for environment %s: %w", envName, err)
	}
	svcParams, err := d.svcDescriber.Params()
	if err != nil {
		return "", fmt.Errorf("get parameters for service %s: %w", d.service.Name, err)
	}

	uri := &WebServiceURI{
		DNSName: envOutputs[stack.EnvOutputPublicLoadBalancerDNSName],
		Path:    svcParams[stack.LBWebServiceRulePathParamKey],
	}
	_, isHTTPS := envOutputs[stack.EnvOutputSubdomain]
	if isHTTPS {
		dnsName := fmt.Sprintf("%s.%s", d.service.Name, envOutputs[stack.EnvOutputSubdomain])
		uri = &WebServiceURI{
			DNSName: dnsName,
		}
	}
	return uri.String(), nil
}

// EnvVars contains serialized environment variables for a service.
type EnvVars struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

type envVars []*EnvVars

func (e envVars) humanString(w io.Writer) {
	fmt.Fprintf(w, "  %s\t%s\t%s\n", "Name", "Environment", "Value")
	var prevName string
	var prevValue string
	for _, variable := range e {
		// Instead of re-writing the same variable value, we replace it with "-" to reduce text.
		if variable.Name != prevName {
			if variable.Value != prevValue {
				fmt.Fprintf(w, "  %s\t%s\t%s\n", variable.Name, variable.Environment, variable.Value)
			} else {
				fmt.Fprintf(w, "  %s\t%s\t-\n", variable.Name, variable.Environment)
			}
		} else {
			if variable.Value != prevValue {
				fmt.Fprintf(w, "  -\t%s\t%s\n", variable.Environment, variable.Value)
			} else {
				fmt.Fprintf(w, "  -\t%s\t-\n", variable.Environment)
			}
		}
		prevName = variable.Name
		prevValue = variable.Value
	}
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
	fmt.Fprintf(w, "  %s\t%s\n", "Environment", "Namespace")
	for _, sd := range s {
		fmt.Fprintf(w, "  %s\t%s\n", strings.Join(sd.Environment, ", "), sd.Namespace)
	}
}

// webSvcDesc contains serialized parameters for a web service.
type webSvcDesc struct {
	Service          string             `json:"service"`
	Type             string             `json:"type"`
	App              string             `json:"application"`
	Configurations   configurations     `json:"configurations"`
	Routes           []*WebServiceRoute `json:"routes"`
	ServiceDiscovery serviceDiscoveries `json:"serviceDiscovery"`
	Variables        envVars            `json:"variables"`
	Resources        cfnResources       `json:"resources,omitempty"`
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
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Application", w.App)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", w.Service)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", w.Type)
	fmt.Fprintf(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer.Flush()
	w.Configurations.humanString(writer)
	fmt.Fprintf(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Environment", "URL")
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
	}
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
		// Show the resources by the order of environments displayed under Configuration for a consistent view.
		w.Resources.humanStringByEnv(writer, w.Configurations)
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
	for {
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
