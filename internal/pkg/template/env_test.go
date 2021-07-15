// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
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
		"renders env template": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("environment/cf.yml", "test")
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

			// WHEN
			c, err := tpl.ParseEnv(&EnvOpts{})

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}
