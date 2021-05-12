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
		version          string
		mockDependencies func(t *Template)
		wantedContent    string
		wantedErr        error
	}{
		"renders all nested templates in legacy template": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				var baseContent string
				for _, name := range envCFSubTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("environment/versions/cf-v0.0.0.yml", baseContent)
				t.box = mockBox
			},
			wantedContent: `  cfn-execution-role
  custom-resources
  custom-resources-role
  environment-manager-role
  lambdas
  vpc-resources
  nat-gateways
  vpc-endpoints
`,
		},
		"renders v1.0.0 template": {
			version: "v1.0.0",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("environment/versions/cf-v1.0.0.yml", "test")
				t.box = mockBox
			},
			wantedContent: "test",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)
			tpl.box.AddString("environment/partials/cfn-execution-role.yml", "cfn-execution-role")
			tpl.box.AddString("environment/partials/custom-resources.yml", "custom-resources")
			tpl.box.AddString("environment/partials/custom-resources-role.yml", "custom-resources-role")
			tpl.box.AddString("environment/partials/environment-manager-role.yml", "environment-manager-role")
			tpl.box.AddString("environment/partials/lambdas.yml", "lambdas")
			tpl.box.AddString("environment/partials/vpc-resources.yml", "vpc-resources")
			tpl.box.AddString("environment/partials/nat-gateways.yml", "nat-gateways")
			tpl.box.AddString("environment/partials/vpc-endpoints.yml", "vpc-endpoints")

			// WHEN
			c, err := tpl.ParseEnv(&EnvOpts{
				Version: tc.version,
			})

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}
