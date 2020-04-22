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

// Names of application templates.
const (
	lbWebAppTemplateName   = "lb-web-app"
	backendAppTemplateName = "backend-app"
)

// AppNestedStackOpts holds configuration that's needed if the application stack has a nested stack.
type AppNestedStackOpts struct {
	StackName string

	VariableOutputs []string
	SecretOutputs   []string
	PolicyOutputs   []string
}

// AppHealthCheckOpts holds configuration for the container healthcheck.
type AppHealthCheckOpts struct {
	Command     []string
	Interval    int
	Retries     int
	StartPeriod int
	Timeout     int
}

// AppOpts holds optional data that can be provided to enable features in an application stack template.
type AppOpts struct {
	Variables map[string]string
	Secrets   map[string]string

	HealthCheck *ecs.HealthCheck

	// Outputs from nested stacks such as the addons stack.
	NestedStack *AppNestedStackOpts

	// Custom resource lambdas.
	RulePriorityLambda string
}

// ParseLoadBalancedWebApp parses a load balanced web app's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseLoadBalancedWebApp(data AppOpts) (*Content, error) {
	return t.ParseApp(lbWebAppTemplateName, data, withAppParsingFuncs())
}

// ParseBackendApp parses a backend app's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseBackendApp(data AppOpts) (*Content, error) {
	return t.ParseApp(backendAppTemplateName, data, withAppParsingFuncs())
}

// ParseApp parses an application's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParseApp(name string, data interface{}, options ...ParseOption) (*Content, error) {
	tpl, err := t.parse("base", fmt.Sprintf(fmtAppCFTemplatePath, name), options...)
	if err != nil {
		return nil, err
	}
	for _, templateName := range commonCFTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtCommonCFTemplatePath, templateName), options...)
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
		return nil, fmt.Errorf("execute app template %s with data %v: %w", name, data, err)
	}
	return &Content{buf}, nil
}

func withAppParsingFuncs() ParseOption {
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

func hasSecrets(opts AppOpts) bool {
	if opts.Secrets != nil {
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
