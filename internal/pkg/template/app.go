// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
)

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
