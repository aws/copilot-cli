// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/ecs"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

const (
	// Display settings.
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

// WebAppURI represents the unique identifier to access a web application.
type WebAppURI struct {
	DNSName string // The environment's subdomain if the application is served on HTTPS. Otherwise, the public load balancer's DNS.
	Path    string // Empty if the application is served on HTTPS. Otherwise, the pattern used to match the application.
}

func (uri *WebAppURI) String() string {
	switch uri.Path {
	// When the app is using host based routing, the app
	// is included in the DNS name (app.myenv.myproj.dns.com)
	case "":
		return fmt.Sprintf("https://%s", uri.DNSName)
	// When the app is using the root path, there is no "path"
	// (for example http://lb.us-west-2.amazon.com/)
	case "/":
		return fmt.Sprintf("http://%s", uri.DNSName)
	// Otherwise, if there is a path for the app, link to the
	// LoadBalancer DNS name and the path
	// (for example http://lb.us-west-2.amazon.com/app)
	default:
		return fmt.Sprintf("http://%s/%s", uri.DNSName, uri.Path)
	}
}

type storeSvc interface {
	GetEnvironment(projectName string, environmentName string) (*archer.Environment, error)
	ListEnvironments(projectName string) ([]*archer.Environment, error)
}

type appDescriber interface {
	Params() (map[string]string, error)
	EnvOutputs() (map[string]string, error)
	EnvVars() (map[string]string, error)
	GetServiceArn() (*ecs.ServiceArn, error)
	AppStackResources() ([]*cloudformation.StackResource, error)
}

// WebAppDescriber retrieves information about a load balanced web application.
type WebAppDescriber struct {
	app             *archer.Application
	enableResources bool

	store            storeSvc
	appDescriber     appDescriber
	initAppDescriber func(string) error
}

// NewWebAppDescriber instantiates a load balanced application.
func NewWebAppDescriber(project, app string) (*WebAppDescriber, error) {
	svc, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	meta, err := svc.GetApplication(project, app)
	if err != nil {
		return nil, err
	}
	opts := &WebAppDescriber{
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

// NewWebAppDescriberWithResources instantiates a load balanced application with stack resources.
func NewWebAppDescriberWithResources(project, app string) (*WebAppDescriber, error) {
	d, err := NewWebAppDescriber(project, app)
	if err != nil {
		return nil, err
	}
	d.enableResources = true
	return d, nil
}

// Describe returns info of a web app application.
func (d *WebAppDescriber) Describe() (*WebAppDesc, error) {
	environments, err := d.store.ListEnvironments(d.app.Project)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var routes []*WebAppRoute
	var configs []*WebAppConfig
	var envVars []*EnvVars
	for _, env := range environments {
		err := d.initAppDescriber(env.Name)
		if err != nil {
			return nil, err
		}
		webAppURI, err := d.URI(env.Name)
		if err == nil {
			routes = append(routes, &WebAppRoute{
				Environment: env.Name,
				URL:         webAppURI,
			})
			appParams, err := d.appDescriber.Params()
			if err != nil {
				return nil, fmt.Errorf("retrieve application deployment configuration: %w", err)
			}
			configs = append(configs, &WebAppConfig{
				Environment: env.Name,
				Port:        appParams[stack.LBWebAppContainerPortParamKey],
				Tasks:       appParams[stack.AppTaskCountParamKey],
				CPU:         appParams[stack.AppTaskCPUParamKey],
				Memory:      appParams[stack.AppTaskMemoryParamKey],
			})
			webAppEnvVars, err := d.appDescriber.EnvVars()
			if err != nil {
				return nil, fmt.Errorf("retrieve environment variables: %w", err)
			}
			envVars = append(envVars, flattenEnvVars(env.Name, webAppEnvVars)...)
			continue
		}
		if !IsStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve application URI: %w", err)
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

	return &WebAppDesc{
		AppName:        d.app.Name,
		Type:           d.app.Type,
		Project:        d.app.Project,
		Configurations: configs,
		Routes:         routes,
		Variables:      envVars,
		Resources:      resources,
	}, nil
}

// URI returns the WebAppURI to identify this application uniquely given an environment name.
func (d *WebAppDescriber) URI(envName string) (string, error) {
	err := d.initAppDescriber(envName)
	if err != nil {
		return "", err
	}

	envOutputs, err := d.appDescriber.EnvOutputs()
	if err != nil {
		return "", fmt.Errorf("get output for environment %s: %w", envName, err)
	}
	appParams, err := d.appDescriber.Params()
	if err != nil {
		return "", fmt.Errorf("get parameters for application %s: %w", d.app.Name, err)
	}

	uri := &WebAppURI{
		DNSName: envOutputs[stack.EnvOutputPublicLoadBalancerDNSName],
		Path:    appParams[stack.LBWebAppRulePathParamKey],
	}
	_, isHTTPS := envOutputs[stack.EnvOutputSubdomain]
	if isHTTPS {
		dnsName := fmt.Sprintf("%s.%s", d.app.Name, envOutputs[stack.EnvOutputSubdomain])
		uri = &WebAppURI{
			DNSName: dnsName,
		}
	}
	return uri.String(), nil
}

// EnvVars contains serialized environment variables for an application.
type EnvVars struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Value       string `json:"value"`
}

// CfnResource contains application resources created by cloudformation.
type CfnResource struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
}

// WebAppConfig contains serialized configuration parameters for a web application.
type WebAppConfig struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
}

// WebAppRoute contains serialized route parameters for a web application.
type WebAppRoute struct {
	Environment string `json:"environment"`
	URL         string `json:"url"`
}

// WebAppDesc contains serialized parameters for a web application.
type WebAppDesc struct {
	AppName        string                    `json:"appName"`
	Type           string                    `json:"type"`
	Project        string                    `json:"project"`
	Configurations []*WebAppConfig           `json:"configurations"`
	Routes         []*WebAppRoute            `json:"routes"`
	Variables      []*EnvVars                `json:"variables"`
	Resources      map[string][]*CfnResource `json:"resources,omitempty"`
}

// JSONString returns the stringified WebApp struct with json format.
func (w *WebAppDesc) JSONString() (string, error) {
	b, err := json.Marshal(w)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified WebApp struct with human readable format.
func (w *WebAppDesc) HumanString() string {
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
	fmt.Fprintf(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Environment", "URL")
	for _, route := range w.Routes {
		fmt.Fprintf(writer, "  %s\t%s\n", route.Environment, route.URL)
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
		for _, route := range w.Routes {
			env := route.Environment
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
