// +build integration

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseLoadBalancedWebService(t *testing.T) {
	testCases := map[string]struct {
		opts template.ServiceOpts
	}{
		"renders a valid template by default": {
			opts: template.ServiceOpts{},
		},
		"renders a valid template with addons with no outputs": {
			opts:  template.ServiceOpts{
				NestedStack: &template.ServiceNestedStackOpts{
					StackName: "AddonsStack",
				},
			},
		},
		"renders a valid template with addons with outputs": {
			opts:  template.ServiceOpts{
				NestedStack: &template.ServiceNestedStackOpts{
					StackName:       "AddonsStack",
					VariableOutputs: []string{"TableName"},
					SecretOutputs:   []string{"TablePassword"},
					PolicyOutputs:   []string{"TablePolicy"},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sess, err := session.NewProvider().Default()
			require.NoError(t, err)
			cfn := cloudformation.New(sess)
			tpl := template.New()

			// WHEN
			content, err := tpl.ParseLoadBalancedWebService(tc.opts)
			require.NoError(t, err)

			// THEN
			_, err = cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
				TemplateBody: aws.String(content.String()),
			})
			require.NoError(t, err, content.String())
		})
	}
}