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
				for _, name := range partialsWorkloadCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}
				mockBox.AddString("workloads/services/backend/cf.yml", baseContent)
				mockBox.AddString("workloads/partials/cf/loggroup.yml", "loggroup")
				mockBox.AddString("workloads/partials/cf/envvars.yml", "envvars")
				mockBox.AddString("workloads/partials/cf/secrets.yml", "secrets")
				mockBox.AddString("workloads/partials/cf/executionrole.yml", "executionrole")
				mockBox.AddString("workloads/partials/cf/taskrole.yml", "taskrole")
				mockBox.AddString("workloads/partials/cf/fargate-taskdef-base-properties.yml", "fargate-taskdef-base-properties")
				mockBox.AddString("workloads/partials/cf/service-base-properties.yml", "service-base-properties")
				mockBox.AddString("workloads/partials/cf/servicediscovery.yml", "servicediscovery")
				mockBox.AddString("workloads/partials/cf/addons.yml", "addons")
				mockBox.AddString("workloads/partials/cf/sidecars.yml", "sidecars")
				mockBox.AddString("workloads/partials/cf/logconfig.yml", "logconfig")
				mockBox.AddString("workloads/partials/cf/autoscaling.yml", "autoscaling")
				mockBox.AddString("workloads/partials/cf/state-machine-definition.json.yml", "state-machine-definition")
				mockBox.AddString("workloads/partials/cf/eventrule.yml", "eventrule")
				mockBox.AddString("workloads/partials/cf/state-machine.yml", "state-machine")
				mockBox.AddString("workloads/partials/cf/env-controller.yml", "env-controller")
				mockBox.AddString("workloads/partials/cf/mount-points.yml", "mount-points")
				mockBox.AddString("workloads/partials/cf/volumes.yml", "volumes")
				mockBox.AddString("workloads/partials/cf/image-overrides.yml", "image-overrides")

				t.box = mockBox
			},
			wantedContent: `  loggroup
  envvars
  secrets
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
  mount-points
  volumes
  image-overrides
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
