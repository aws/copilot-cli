// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"gopkg.in/yaml.v3"
)

// CFNSelector represents a selector for a CloudFormation template.
type CFNSelector struct {
	prompt Prompter
}

// NewCFNSelector initializes a CFNSelector.
func NewCFNSelector(prompt Prompter) *CFNSelector {
	return &CFNSelector{
		prompt: prompt,
	}
}

// Resources prompts the user to multiselect resources from a CloudFormation template body.
// By default, the prompt filters out any custom resource in the template.
func (sel *CFNSelector) Resources(msg, finalMsg, help, body string) ([]template.CFNResource, error) {
	tpl := struct {
		Resources map[string]struct {
			Type string `yaml:"Type"`
		} `yaml:"Resources"`
	}{}
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal CloudFormation template: %v", err)
	}

	// Prompt for a selection.
	var options []prompt.Option
	for name, resource := range tpl.Resources {
		if resource.Type == "AWS::Lambda::Function" || strings.HasPrefix(resource.Type, "Custom::") {
			continue
		}
		options = append(options, prompt.Option{
			Value: name,
			Hint:  resource.Type,
		})
	}
	sort.Slice(options, func(i, j int) bool { // Sort options by resource type, if they're the same resource type then sort by logicalID.
		if options[i].Hint == options[j].Hint {
			return options[i].Value < options[j].Value
		}
		return options[i].Hint < options[j].Hint
	})
	logicalIDs, err := sel.prompt.MultiSelectOptions(msg, help, options, prompt.WithFinalMessage(finalMsg))
	if err != nil {
		return nil, fmt.Errorf("select CloudFormation resources: %v", err)
	}

	// Transform to template.CFNResource
	out := make([]template.CFNResource, len(logicalIDs))
	for i, logicalID := range logicalIDs {
		out[i] = template.CFNResource{
			Type:      template.CFNType(tpl.Resources[logicalID].Type),
			LogicalID: logicalID,
		}
	}
	return out, nil
}
