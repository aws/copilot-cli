// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"text/template"
)

const (
	pipelineCFTemplatePath  = "cicd/pipeline_cfn.yml"
	fmtPipelinePartialsPath = "cicd/partials/%s.yml"
)

var pipelinePartialTemplateNames = []string{"build-action", "role-policy-document", "role-config", "actions", "action-config", "test"}

// ParsePipeline parses a pipeline's CloudFormation template with the specified data object and returns its content.
func (t *Template) ParsePipeline(data interface{}) (*Content, error) {
	tpl, err := t.parse("base", pipelineCFTemplatePath, withPipelineParsingFuncs())
	if err != nil {
		return nil, err
	}
	for _, templateName := range pipelinePartialTemplateNames {
		nestedTpl, err := t.parse(templateName, fmt.Sprintf(fmtPipelinePartialsPath, templateName), withPipelineParsingFuncs())
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
		return nil, fmt.Errorf("execute pipeline template with data %v: %w", data, err)
	}
	return &Content{buf}, nil
}

func withPipelineParsingFuncs() ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(map[string]interface{}{
			"isCodeStarConnection": func(source interface{}) bool {
				type connectionName interface {
					ConnectionName() (string, error)
				}
				_, ok := source.(connectionName)
				return ok
			},
			"logicalIDSafe": ReplaceDashesFunc,
			"alphanumeric":  StripNonAlphaNumFunc,
		})
	}
}
