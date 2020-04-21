// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseAppTemplate(t *testing.T) {
	const (
		testAppName = "backend-app"
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
				for _, name := range commonCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("applications/backend-app/cf.yml", baseContent)
				mockBox.AddString("applications/common/cf/loggroup.yml", "loggroup")
				mockBox.AddString("applications/common/cf/envvars.yml", "envvars")
				mockBox.AddString("applications/common/cf/executionrole.yml", "executionrole")
				mockBox.AddString("applications/common/cf/taskrole.yml", "taskrole")
				mockBox.AddString("applications/common/cf/fargate-taskdef-base-properties.yml", "fargate-taskdef-base-properties")
				mockBox.AddString("applications/common/cf/service-base-properties.yml", "service-base-properties")
				mockBox.AddString("applications/common/cf/servicediscovery.yml", "servicediscovery")
				mockBox.AddString("applications/common/cf/addons.yml", "addons")

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
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.ParseApp(testAppName, nil)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"camel case: starts with uppercase": {
			in:     "AdditionalResourcesPolicyArn",
			wanted: "ADDITIONAL_RESOURCES_POLICY_ARN",
		},
		"camel case: starts with lowercase": {
			in:     "additionalResourcesPolicyArn",
			wanted: "ADDITIONAL_RESOURCES_POLICY_ARN",
		},
		"all lower case": {
			in:     "myddbtable",
			wanted: "MYDDBTABLE",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, toSnakeCase(tc.in))
		})
	}
}

func TestHasSecrets(t *testing.T) {
	testCases := map[string]struct {
		in     AppOpts
		wanted bool
	}{
		"no secrets": {
			in:     AppOpts{},
			wanted: false,
		},
		"app has secrets": {
			in: AppOpts{
				Secrets: map[string]string{
					"hello": "world",
				},
			},
			wanted: true,
		},
		"nested has secrets": {
			in: AppOpts{
				NestedStack: &AppNestedStackOpts{
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

func TestStringifySlice(t *testing.T) {
	require.Equal(t, "[]", stringifySlice(nil))
	require.Equal(t, "[a]", stringifySlice([]string{"a"}))
	require.Equal(t, "[a, b, c]", stringifySlice([]string{"a", "b", "c"}))
}

func TestQuoteAll(t *testing.T) {
	require.Equal(t, []string(nil), quoteAll(nil))
	require.Equal(t, []string{`"a"`}, quoteAll([]string{"a"}))
	require.Equal(t, []string{`"a"`, `"b"`, `"c"`}, quoteAll([]string{"a", "b", "c"}))
}
