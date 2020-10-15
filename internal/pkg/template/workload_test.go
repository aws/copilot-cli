// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
				for _, name := range commonWorkloadCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("workloads/services/backend/cf.yml", baseContent)
				mockBox.AddString("workloads/common/cf/loggroup.yml", "loggroup")
				mockBox.AddString("workloads/common/cf/envvars.yml", "envvars")
				mockBox.AddString("workloads/common/cf/executionrole.yml", "executionrole")
				mockBox.AddString("workloads/common/cf/taskrole.yml", "taskrole")
				mockBox.AddString("workloads/common/cf/fargate-taskdef-base-properties.yml", "fargate-taskdef-base-properties")
				mockBox.AddString("workloads/common/cf/service-base-properties.yml", "service-base-properties")
				mockBox.AddString("workloads/common/cf/servicediscovery.yml", "servicediscovery")
				mockBox.AddString("workloads/common/cf/addons.yml", "addons")
				mockBox.AddString("workloads/common/cf/sidecars.yml", "sidecars")
				mockBox.AddString("workloads/common/cf/logconfig.yml", "logconfig")
				mockBox.AddString("workloads/common/cf/autoscaling.yml", "autoscaling")
				mockBox.AddString("workloads/common/cf/state-machine-definition.json.yml", "state-machine-definition")
				mockBox.AddString("workloads/common/cf/eventrule.yml", "eventrule")
				mockBox.AddString("workloads/common/cf/state-machine.yml", "state-machine")
				mockBox.AddString("workloads/common/cf/env-controller.yml", "env-controller")

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
  autoscaling
  eventrule
  state-machine
  state-machine-definition
  env-controller
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
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestHasSecrets(t *testing.T) {
	testCases := map[string]struct {
		in     WorkloadOpts
		wanted bool
	}{
		"nil secrets": {
			in:     WorkloadOpts{},
			wanted: false,
		},
		"no secrets": {
			in: WorkloadOpts{
				Secrets: map[string]string{},
			},
			wanted: false,
		},
		"service has secrets": {
			in: WorkloadOpts{
				Secrets: map[string]string{
					"hello": "world",
				},
			},
			wanted: true,
		},
		"nested has secrets": {
			in: WorkloadOpts{
				NestedStack: &WorkloadNestedStackOpts{
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
