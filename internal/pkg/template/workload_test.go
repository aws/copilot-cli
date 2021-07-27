// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
				mockBox.AddString("workloads/partials/cf/workload-container.yml", "workload-container")
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
				mockBox.AddString("workloads/partials/cf/efs-access-point.yml", "efs-access-point")
				mockBox.AddString("workloads/partials/cf/env-controller.yml", "env-controller")
				mockBox.AddString("workloads/partials/cf/mount-points.yml", "mount-points")
				mockBox.AddString("workloads/partials/cf/volumes.yml", "volumes")
				mockBox.AddString("workloads/partials/cf/image-overrides.yml", "image-overrides")
				mockBox.AddString("workloads/partials/cf/instancerole.yml", "instancerole")
				mockBox.AddString("workloads/partials/cf/accessrole.yml", "accessrole")
				mockBox.AddString("workloads/partials/cf/publish.yml", "publish")

				t.box = mockBox
			},
			wantedContent: `  loggroup
  envvars
  secrets
  executionrole
  taskrole
  workload-container
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
  efs-access-point
  env-controller
  mount-points
  volumes
  image-overrides
  instancerole
  accessrole
  publish
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

func TestTemplate_ParseNetwork(t *testing.T) {
	type cfn struct {
		Resources struct {
			Service struct {
				Properties struct {
					NetworkConfiguration map[interface{}]interface{} `yaml:"NetworkConfiguration"`
				} `yaml:"Properties"`
			} `yaml:"Service"`
		} `yaml:"Resources"`
	}

	testCases := map[string]struct {
		input *NetworkOpts

		wantedNetworkConfig string
	}{
		"should render AWS VPC configuration for public subnets by default": {
			input: nil,
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: ENABLED
    Subnets:
      Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PublicSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
`,
		},
		"should render AWS VPC configuration for private subnets": {
			input: &NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
			},
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: DISABLED
    Subnets:
      Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
`,
		},
		"should render AWS VPC configuration for private subnets with security groups": {
			input: &NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
				SecurityGroups: []string{
					"sg-1bcf1d5b",
					"sg-asdasdas",
				},
			},
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: DISABLED
    Subnets:
      Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
      - "sg-1bcf1d5b"
      - "sg-asdasdas"
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := New()
			wanted := make(map[interface{}]interface{})
			err := yaml.Unmarshal([]byte(tc.wantedNetworkConfig), &wanted)
			require.NoError(t, err, "unmarshal wanted config")

			// WHEN
			content, err := tpl.ParseLoadBalancedWebService(WorkloadOpts{
				Network: tc.input,
			})

			// THEN
			require.NoError(t, err, "parse load balanced web service")
			var actual cfn
			err = yaml.Unmarshal(content.Bytes(), &actual)
			require.NoError(t, err, "unmarshal actual config")
			require.Equal(t, wanted, actual.Resources.Service.Properties.NetworkConfiguration)
		})
	}
}
