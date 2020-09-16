// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseEnv(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *Template)
		wantedContent    string
		wantedErr        error
	}{
		"renders all nested templates": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				var baseContent string
				for _, name := range envCFSubTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("environment/versions/cf-legacy.yml", baseContent)
				mockBox.AddString("environment/partials/cfn-execution-role.yml", "cfn-execution-role")
				mockBox.AddString("environment/partials/custom-resources.yml", "custom-resources")
				mockBox.AddString("environment/partials/custom-resources-role.yml", "custom-resources-role")
				mockBox.AddString("environment/partials/environment-manager-role.yml", "environment-manager-role")
				mockBox.AddString("environment/partials/lambdas.yml", "lambdas")
				mockBox.AddString("environment/partials/vpc-resources.yml", "vpc-resources")

				t.box = mockBox
			},
			wantedContent: `  cfn-execution-role
  custom-resources
  custom-resources-role
  environment-manager-role
  lambdas
  vpc-resources
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.ParseEnv(nil)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}
