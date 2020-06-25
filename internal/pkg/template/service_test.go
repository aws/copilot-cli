// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseSvc(t *testing.T) {
	const (
		testSvcName = "backend"
	)
	testCases := map[string]struct {
		mockDependencies func(t *Template)
		wantedContent    string
		wantedErr        error
	}{
		"renders all common templates": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				var baseContent string
				for _, name := range commonServiceCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("services/backend/cf.yml", baseContent)
				mockBox.AddString("services/common/cf/loggroup.yml", "loggroup")
				mockBox.AddString("services/common/cf/envvars.yml", "envvars")
				mockBox.AddString("services/common/cf/executionrole.yml", "executionrole")
				mockBox.AddString("services/common/cf/taskrole.yml", "taskrole")
				mockBox.AddString("services/common/cf/fargate-taskdef-base-properties.yml", "fargate-taskdef-base-properties")
				mockBox.AddString("services/common/cf/service-base-properties.yml", "service-base-properties")
				mockBox.AddString("services/common/cf/servicediscovery.yml", "servicediscovery")
				mockBox.AddString("services/common/cf/addons.yml", "addons")
				mockBox.AddString("services/common/cf/sidecars.yml", "sidecars")
				mockBox.AddString("services/common/cf/logconfig.yml", "logconfig")

				t.box = mockBox
			},
			wantedContent: `  loggroup
  envvars
  executionrole
  taskrole
  fargate-taskdef-base-properties
  service-base-properties
  servicediscovery
  addons
  sidecars
  logconfig
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.parseSvc(testSvcName, nil)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestHasSecrets(t *testing.T) {
	testCases := map[string]struct {
		in     ServiceOpts
		wanted bool
	}{
		"nil secrets": {
			in:     ServiceOpts{},
			wanted: false,
		},
		"no secrets": {
			in: ServiceOpts{
				Secrets: map[string]string{},
			},
			wanted: false,
		},
		"service has secrets": {
			in: ServiceOpts{
				Secrets: map[string]string{
					"hello": "world",
				},
			},
			wanted: true,
		},
		"nested has secrets": {
			in: ServiceOpts{
				NestedStack: &ServiceNestedStackOpts{
					SecretOutputs: []string{"MySecretArn"},
				},
			},
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, hasSecrets(tc.in))
		})
	}
}
