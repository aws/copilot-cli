// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/google/uuid"
)

// Paths of workload cloudformation templates under templates/workloads/.
const (
	fmtWkldCFTemplatePath       = "workloads/%s/%s/cf.yml"
	fmtWkldCommonCFTemplatePath = "workloads/common/cf/%s.yml"
)

const (
	servicesDirName = "services"
)

var (
	// Template names under "workloads/common/cf/".
	commonWorkloadCFTemplateNames = []string{
		"loggroup",
		"envvars",
		"executionrole",
		"taskrole",
		"fargate-taskdef-base-properties",
		"service-base-properties",
		"servicediscovery",
		"addons",
		"sidecars",
		"logconfig",
		"autoscaling",
	}
)

// Names of service templates.
const (
	lbWebSvcTplName   = "lb-web"
	backendSvcTplName = "backend"
)

// ServiceNestedStackOpts holds configuration that's needed if the service stack has a nested stack.
type ServiceNestedStackOpts struct {
	StackName string

	VariableOutputs []string
	SecretOutputs   []string
	PolicyOutputs   []string
}

// SidecarOpts holds configuration that's needed if the service has sidecar containers.
type SidecarOpts struct {
	Name       *string
	Image      *string
	Port       *string
	Protocol   *string
	CredsParam *string
}

// LogConfigOpts holds configuration that's needed if the service is configured with Firelens to route
// its logs.
type LogConfigOpts struct {
	Image          *string
	Destination    map[string]string
	EnableMetadata *string
	SecretOptions  map[string]string
	ConfigFile     *string
}

// AutoscalingOpts holds configuration that's needed for Auto Scaling.
type AutoscalingOpts struct {
	MinCapacity  *int
	MaxCapacity  *int
	CPU          *float64
	Memory       *float64
	Requests     *float64
	ResponseTime *float64
}

// ServiceOpts holds optional data that can be provided to enable features in a service stack template.
type ServiceOpts struct {
	// Additional options that're common between **all** service templates.
	Variables   map[string]string
	Secrets     map[string]string
	NestedStack *ServiceNestedStackOpts // Outputs from nested stacks such as the addons stack.
	Sidecars    []*SidecarOpts
	LogConfig   *LogConfigOpts
	Autoscaling *AutoscalingOpts

	// Additional options that're not shared across all service templates.
	HealthCheck        *ecs.HealthCheck
	RulePriorityLambda string
	DesiredCountLambda string
}

// ParseLoadBalancedWebService parses a load balanced web service's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseLoadBalancedWebService(data ServiceOpts) (*Content, error) {
	return t.parseSvc(lbWebSvcTplName, data, withSvcParsingFuncs())
}

// ParseBackendService parses a backend service's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseBackendService(data ServiceOpts) (*Content, error) {
	return t.parseSvc(backendSvcTplName, data, withSvcParsingFuncs())
}

// parseSvc parses a service's CloudFormation template with the specified data object and returns its content.
func (t *Template) parseSvc(name string, data interface{}, options ...ParseOption) (*Content, error) {
	tpl, err := t.parse("base", fmt.Sprintf(fmtWkldCFTemplatePath, servicesDirName, name), options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range commonWorkloadCFTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtWkldCommonCFTemplatePath, templateName), options...)
		if err != nil {
			return nil, err
		}
		_, err = tpl.AddParseTree(templateName, nestedTpl.Tree)
		if err != nil {
			return nil, fmt.Errorf("add parse tree of %s to base template: %w", templateName, err)
		}
	}
	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute service template %s with data %v: %w", name, data, err)
	}
	return &Content{buf}, nil
}

func withSvcParsingFuncs() ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(map[string]interface{}{
			"toSnakeCase": ToSnakeCaseFunc,
			"hasSecrets":  hasSecrets,
			"fmtSlice":    FmtSliceFunc,
			"quoteSlice":  QuotePSliceFunc,
			"randomUUID":  randomUUIDFunc,
		})
	}
}

func hasSecrets(opts ServiceOpts) bool {
	if len(opts.Secrets) > 0 {
		return true
	}
	if opts.NestedStack != nil && (len(opts.NestedStack.SecretOutputs) > 0) {
		return true
	}
	return false
}

func randomUUIDFunc() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("generate random uuid: %w", err)
	}
	return id.String(), err
}
