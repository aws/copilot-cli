// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTemplate_ParseSvc(t *testing.T) {
	const (
		testSvcName = "backend"
	)
	testCases := map[string]struct {
		fs            func() map[string][]byte
		wantedContent string
		wantedErr     error
	}{
		"renders all common templates": {
			fs: func() map[string][]byte {
				var baseContent string
				for _, name := range partialsWorkloadCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}

				return map[string][]byte{
					"templates/workloads/services/backend/cf.yml":                         []byte(baseContent),
					"templates/workloads/partials/cf/loggroup.yml":                        []byte("loggroup"),
					"templates/workloads/partials/cf/envvars-container.yml":               []byte("envvars-container"),
					"templates/workloads/partials/cf/envvars-common.yml":                  []byte("envvars-common"),
					"templates/workloads/partials/cf/secrets.yml":                         []byte("secrets"),
					"templates/workloads/partials/cf/executionrole.yml":                   []byte("executionrole"),
					"templates/workloads/partials/cf/taskrole.yml":                        []byte("taskrole"),
					"templates/workloads/partials/cf/workload-container.yml":              []byte("workload-container"),
					"templates/workloads/partials/cf/fargate-taskdef-base-properties.yml": []byte("fargate-taskdef-base-properties"),
					"templates/workloads/partials/cf/service-base-properties.yml":         []byte("service-base-properties"),
					"templates/workloads/partials/cf/servicediscovery.yml":                []byte("servicediscovery"),
					"templates/workloads/partials/cf/addons.yml":                          []byte("addons"),
					"templates/workloads/partials/cf/sidecars.yml":                        []byte("sidecars"),
					"templates/workloads/partials/cf/logconfig.yml":                       []byte("logconfig"),
					"templates/workloads/partials/cf/autoscaling.yml":                     []byte("autoscaling"),
					"templates/workloads/partials/cf/state-machine-definition.json.yml":   []byte("state-machine-definition"),
					"templates/workloads/partials/cf/eventrule.yml":                       []byte("eventrule"),
					"templates/workloads/partials/cf/state-machine.yml":                   []byte("state-machine"),
					"templates/workloads/partials/cf/efs-access-point.yml":                []byte("efs-access-point"),
					"templates/workloads/partials/cf/env-controller.yml":                  []byte("env-controller"),
					"templates/workloads/partials/cf/mount-points.yml":                    []byte("mount-points"),
					"templates/workloads/partials/cf/volumes.yml":                         []byte("volumes"),
					"templates/workloads/partials/cf/image-overrides.yml":                 []byte("image-overrides"),
					"templates/workloads/partials/cf/instancerole.yml":                    []byte("instancerole"),
					"templates/workloads/partials/cf/accessrole.yml":                      []byte("accessrole"),
					"templates/workloads/partials/cf/publish.yml":                         []byte("publish"),
					"templates/workloads/partials/cf/subscribe.yml":                       []byte("subscribe"),
					"templates/workloads/partials/cf/nlb.yml":                             []byte("nlb"),
				}
			},
			wantedContent: `  loggroup
  envvars-container
  envvars-common
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
  subscribe
  nlb
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockReadFileFS{tc.fs()},
			}

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

func TestRuntimePlatformOpts_Version(t *testing.T) {
	testCases := map[string]struct {
		in       RuntimePlatformOpts
		wantedPV string
	}{
		"should return LATEST for on empty platform": {
			wantedPV: "LATEST",
		},
		"should return LATEST for linux containers": {
			in: RuntimePlatformOpts{
				OS:   "LINUX",
				Arch: "X86_64",
			},
			wantedPV: "LATEST",
		},
		"should return 1.0.0 for windows containers": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_FULL",
				Arch: "X86_64",
			},
			wantedPV: "1.0.0",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wantedPV, tc.in.Version())
		})
	}
}

func TestRuntimePlatformOpts_IsDefault(t *testing.T) {
	testCases := map[string]struct {
		in     RuntimePlatformOpts
		wanted bool
	}{
		"should return true on empty platform": {
			wanted: true,
		},
		"should return true for linux/x86_64": {
			in: RuntimePlatformOpts{
				OS:   "LINUX",
				Arch: "X86_64",
			},
			wanted: true,
		},
		"should return false for windows containers": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_CORE",
				Arch: "X86_64",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.IsDefault())
		})
	}
}
