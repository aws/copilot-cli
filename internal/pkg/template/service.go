// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/aws/aws-sdk-go/service/ecs"
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

// ServiceOpts holds optional data that can be provided to enable features in a service stack template.
type ServiceOpts struct {
	// Additional options that're common between **all** service templates.
	Variables   map[string]string
	Secrets     map[string]string
	NestedStack *ServiceNestedStackOpts // Outputs from nested stacks such as the addons stack.

	// Additional options that're not shared across all service templates.
	HealthCheck        *ecs.HealthCheck
	RulePriorityLambda string
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
	tpl, err := t.parse("base", fmt.Sprintf(fmtSvcCFTemplatePath, name), options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range commonServiceCFTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtSvcCommonCFTemplatePath, templateName), options...)
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
			"toSnakeCase":    toSnakeCase,
			"hasSecrets":     hasSecrets,
			"stringifySlice": stringifySlice,
			"quoteAll":       quoteAll,
		})
	}
}

// toSnakeCase transforms a CamelCase input string s into an upper SNAKE_CASE string and returns it.
// For example, "usersDdbTableName" becomes "USERS_DDB_TABLE_NAME".
func toSnakeCase(s string) string {
	var name string
	for i, r := range s {
		if unicode.IsUpper(r) && i != 0 {
			name += "_"
		}
		name += string(unicode.ToUpper(r))
	}
	return name
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

func stringifySlice(elems []string) string {
	return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
}

func quoteAll(elems []*string) []string {
	if elems == nil {
		return nil
	}

	quotedElems := make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(*el)
	}
	return quotedElems
}
