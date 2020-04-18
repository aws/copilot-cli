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
