// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"text/template"
	"unicode"
)

// Names of application templates.
const (
	lbWebAppTemplateName = "lb-web-app"
)

// AppNestedStackOpts holds optional configuration that's needed only if the application stack has a nested stack.
type AppNestedStackOpts struct {
	StackName string

	VariableOutputs []string
	SecretOutputs   []string
	PolicyOutputs   []string
}

// AppOpts holds optional data that can be provided to render an application stack template.
// The empty AppOpts{} struct means that none of these fields will be rendered in the application template.
type AppOpts struct {
	Variables map[string]string
	Secrets   map[string]string

	// Outputs from nested stacks such as the addons stack.
	NestedStack *AppNestedStackOpts
}

// LoadBalancedWebAppConfig holds data that needs be provided to render a load balanced web app stack template.
type LoadBalancedWebAppConfig struct {
	// Optional fields.
	AppOpts

	// Mandatory fields.
	RulePriorityLambda string
}

// ParseLoadBalancedWebApp parses a load balanced web app's CloudFormation template
// with the specified data object and returns its content.
func (t *Template) ParseLoadBalancedWebApp(data LoadBalancedWebAppConfig) (*Content, error) {
	return t.ParseApp(lbWebAppTemplateName, data, withAppParsingFuncs())
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
			"toSnakeCase": toSnakeCase,
			"hasSecrets":  hasSecrets,
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
