// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestTemplate_ParseSvc(t *testing.T) {
	const (
		testSvcName = "backend"
	)
	testCases := map[string]struct {
		fs            func() afero.Fs
		wantedContent string
		wantedErr     error
	}{
		"renders all common templates": {
			fs: func() afero.Fs {
				var baseContent string
				for _, name := range partialsWorkloadCFTemplateNames {
					baseContent += fmt.Sprintf(`{{include "%s" . | indent 2}}`+"\n", name)
				}

				fs := afero.NewMemMapFs()
				_ = fs.MkdirAll("templates/workloads/services/backend/", 0755)
				_ = fs.MkdirAll("templates/workloads/partials/cf/", 0755)
				_ = afero.WriteFile(fs, "templates/workloads/services/backend/cf.yml", []byte(baseContent), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/loggroup.yml", []byte("loggroup"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/envvars-container.yml", []byte("envvars-container"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/envvars-common.yml", []byte("envvars-common"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/secrets.yml", []byte("secrets"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/executionrole.yml", []byte("executionrole"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/taskrole.yml", []byte("taskrole"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/workload-container.yml", []byte("workload-container"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/fargate-taskdef-base-properties.yml", []byte("fargate-taskdef-base-properties"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/service-base-properties.yml", []byte("service-base-properties"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/servicediscovery.yml", []byte("servicediscovery"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/addons.yml", []byte("addons"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/sidecars.yml", []byte("sidecars"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/logconfig.yml", []byte("logconfig"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/autoscaling.yml", []byte("autoscaling"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/state-machine-definition.json.yml", []byte("state-machine-definition"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/eventrule.yml", []byte("eventrule"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/state-machine.yml", []byte("state-machine"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/efs-access-point.yml", []byte("efs-access-point"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/https-listener.yml", []byte("https-listener"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/http-listener.yml", []byte("http-listener"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/env-controller.yml", []byte("env-controller"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/mount-points.yml", []byte("mount-points"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/variables.yml", []byte("variables"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/volumes.yml", []byte("volumes"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/image-overrides.yml", []byte("image-overrides"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/instancerole.yml", []byte("instancerole"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/accessrole.yml", []byte("accessrole"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/publish.yml", []byte("publish"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/subscribe.yml", []byte("subscribe"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/nlb.yml", []byte("nlb"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/vpc-connector.yml", []byte("vpc-connector"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/alb.yml", []byte("alb"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/rollback-alarms.yml", []byte("rollback-alarms"), 0644)
				_ = afero.WriteFile(fs, "templates/workloads/partials/cf/imported-alb-resources.yml", []byte("imported-alb-resources"), 0644)

				return fs
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
  https-listener
  http-listener
  env-controller
  mount-points
  variables
  volumes
  image-overrides
  instancerole
  accessrole
  publish
  subscribe
  nlb
  vpc-connector
  alb
  rollback-alarms
  imported-alb-resources
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{
				fs: &mockFS{tc.fs()},
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
				Secrets: map[string]Secret{},
			},
			wanted: false,
		},
		"service has secrets": {
			in: WorkloadOpts{
				Secrets: map[string]Secret{
					"hello": SecretFromPlainSSMOrARN("world"),
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
		"should return 1.0.0 for windows containers 2019 Core": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_CORE",
				Arch: "X86_64",
			},
			wantedPV: "1.0.0",
		},
		"should return 1.0.0 for windows containers 2019 Full": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_FULL",
				Arch: "X86_64",
			},
			wantedPV: "1.0.0",
		},
		"should return 1.0.0 for windows containers 2022 Core": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2022_CORE",
				Arch: "X86_64",
			},
			wantedPV: "1.0.0",
		},
		"should return 1.0.0 for windows containers 2022 Full": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2022_FULL",
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
		"should return false for windows containers 2019 Core": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_CORE",
				Arch: "X86_64",
			},
		},
		"should return false for windows containers 2019 Full": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2019_FULL",
				Arch: "X86_64",
			},
		},
		"should return false for windows containers 2022 Core": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2022_CORE",
				Arch: "X86_64",
			},
		},
		"should return false for windows containers 2022 Full": {
			in: RuntimePlatformOpts{
				OS:   "WINDOWS_SERVER_2022_FULL",
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

func TestPlainSSMOrSecretARN_RequiresSub(t *testing.T) {
	require.False(t, plainSSMOrSecretARN{}.RequiresSub(), "plain SSM Parameter Store or secret ARNs do not require !Sub")
}

func TestPlainSSMOrSecretARN_RequiresImport(t *testing.T) {
	require.False(t, plainSSMOrSecretARN{}.RequiresImport(), "plain SSM Parameter Store or secret ARNs do not require !ImportValue")
}

func TestPlainSSMOrSecretARN_ValueFrom(t *testing.T) {
	require.Equal(t, "/github/token", SecretFromPlainSSMOrARN("/github/token").ValueFrom())
}

func TestImportedSSMOrSecretARN_RequiresSub(t *testing.T) {
	require.False(t, importedSSMorSecretARN{}.RequiresSub(), "imported SSM Parameter Store or secret ARNs do not require !Sub")
}

func TestImportedSSMOrSecretARN_RequiresImport(t *testing.T) {
	require.True(t, importedSSMorSecretARN{}.RequiresImport(), "imported SSM Parameter Store or secret ARNs requires !ImportValue")
}

func TestImportedSSMOrSecretARN_ValueFrom(t *testing.T) {
	require.Equal(t, "stack-SSMGHTokenName", SecretFromImportedSSMOrARN("stack-SSMGHTokenName").ValueFrom())
}

func TestSecretsManagerName_RequiresSub(t *testing.T) {
	require.True(t, secretsManagerName{}.RequiresSub(), "secrets referring to a SecretsManager name need to be expanded to a full ARN")
}

func TestSecretsManagerName_RequiresImport(t *testing.T) {
	require.False(t, secretsManagerName{}.RequiresImport(), "secrets referring to a SecretsManager name do not require !ImportValue")
}

func TestSecretsManagerName_Service(t *testing.T) {
	require.Equal(t, "secretsmanager", secretsManagerName{}.Service())
}

func TestSecretsManagerName_ValueFrom(t *testing.T) {
	require.Equal(t, "secret:aes128-1a2b3c", SecretFromSecretsManager("aes128-1a2b3c").ValueFrom())
}

func TestALBListenerRule_HealthCheckProtocol(t *testing.T) {
	testCases := map[string]struct {
		opts     ALBListenerRule
		expected string
	}{
		"target port 80, health check port unset": {
			opts: ALBListenerRule{
				TargetPort: "80",
			},
		},
		"target port 80, health check port 443": {
			opts: ALBListenerRule{
				TargetPort: "80",
				HTTPHealthCheck: HTTPHealthCheckOpts{
					Port: "443",
				},
			},
			expected: "HTTPS",
		},
		"target port 443, health check port unset": {
			opts: ALBListenerRule{
				TargetPort: "443",
			},
			expected: "HTTPS",
		},
		"target port 443, health check port 80": {
			opts: ALBListenerRule{
				TargetPort: "443",
				HTTPHealthCheck: HTTPHealthCheckOpts{
					Port: "80",
				},
			},
			expected: "HTTP",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.opts.HealthCheckProtocol())
		})
	}
}

func TestEnvControllerParameters(t *testing.T) {
	tests := map[string]struct {
		opts     WorkloadOpts
		expected []string
	}{
		"LBWS": {
			opts: WorkloadOpts{
				WorkloadType: "Load Balanced Web Service",
			},
			expected: []string{"Aliases,"},
		},
		"LBWS with ALB": {
			opts: WorkloadOpts{
				WorkloadType: "Load Balanced Web Service",
				ALBEnabled:   true,
			},
			expected: []string{"ALBWorkloads,", "Aliases,"},
		},
		"LBWS with imported ALB": {
			opts: WorkloadOpts{
				WorkloadType: "Load Balanced Web Service",
				ALBEnabled:   true,
				ImportedALB: &ImportedALB{
					Name: "MyExistingALB",
				},
			},
			expected: []string{},
		},
		"LBWS with ALB and private placement": {
			opts: WorkloadOpts{
				WorkloadType: "Load Balanced Web Service",
				ALBEnabled:   true,
				Network: NetworkOpts{
					SubnetsType: PrivateSubnetsPlacement,
				},
			},
			expected: []string{"ALBWorkloads,", "Aliases,", "NATWorkloads,"},
		},
		"LBWS with ALB, private placement, and storage": {
			opts: WorkloadOpts{
				WorkloadType: "Load Balanced Web Service",
				ALBEnabled:   true,
				Network: NetworkOpts{
					SubnetsType: PrivateSubnetsPlacement,
				},
				Storage: &StorageOpts{
					ManagedVolumeInfo: &ManagedVolumeCreationInfo{
						Name: aws.String("hi"),
					},
				},
			},
			expected: []string{"ALBWorkloads,", "Aliases,", "NATWorkloads,", "EFSWorkloads,"},
		},
		"Backend": {
			opts: WorkloadOpts{
				WorkloadType: "Backend Service",
			},
			expected: []string{},
		},
		"Backend with ALB": {
			opts: WorkloadOpts{
				WorkloadType: "Backend Service",
				ALBEnabled:   true,
			},
			expected: []string{"InternalALBWorkloads,"},
		},
		"RDWS": {
			opts: WorkloadOpts{
				WorkloadType: "Request-Driven Web Service",
			},
			expected: []string{},
		},
		"private RDWS": {
			opts: WorkloadOpts{
				WorkloadType: "Request-Driven Web Service",
				Private:      true,
			},
			expected: []string{"AppRunnerPrivateWorkloads,"},
		},
		"private RDWS with imported VPC Endpoint": {
			opts: WorkloadOpts{
				WorkloadType:         "Request-Driven Web Service",
				Private:              true,
				AppRunnerVPCEndpoint: aws.String("vpce-1234"),
			},
			expected: []string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, envControllerParameters(tc.opts))
		})
	}
}

func TestRollingUpdateRollbackConfig_TruncateAlarmName(t *testing.T) {
	testCases := map[string]struct {
		config      RollingUpdateRollbackConfig
		inApp       string
		inEnv       string
		inSvc       string
		inAlarmType string
		expected    string
	}{
		"with no need to truncate": {
			inApp:       "shortAppName",
			inEnv:       "shortEnvName",
			inSvc:       "shortSvcName",
			inAlarmType: "CopilotRollbackMemAlarm",
			expected:    "shortAppName-shortEnvName-shortSvcName-CopilotRollbackMemAlarm",
		},
		"with need to truncate at 76 chars per element": {
			inApp:       "12345678911234567892123456789312345678941234567895123456789612345678971234567898",
			inEnv:       "12345678911234567892123456789312345678941234567895123456789612345678971234567898",
			inSvc:       "12345678911234567892123456789312345678941234567895123456789612345678971234567898",
			inAlarmType: "CopilotRollbackCPUAlarm",
			expected:    "1234567891123456789212345678931234567894123456789512345678961234567897123456-1234567891123456789212345678931234567894123456789512345678961234567897123456-1234567891123456789212345678931234567894123456789512345678961234567897123456-CopilotRollbackCPUAlarm",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.config.TruncateAlarmName(tc.inApp, tc.inEnv, tc.inSvc, tc.inAlarmType))
		})
	}
}

func TestApplicationLoadBalancer_Aliases(t *testing.T) {
	tests := map[string]struct {
		opts     ALBListener
		expected []string
	}{
		"LBWS with multiple listener rules having multiple aliases each": {
			opts: ALBListener{
				Rules: []ALBListenerRule{
					{
						Aliases: []string{
							"testAlias1",
							"testAlias2",
						},
					},
					{
						Aliases: []string{
							"testAlias1",
							"testAlias3",
						},
					},
				},
			},
			expected: []string{"testAlias1", "testAlias2", "testAlias3"},
		},
		"LBWS having no aliases": {
			opts: ALBListener{
				Rules: []ALBListenerRule{{}, {}},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.opts.Aliases())
		})
	}
}

func Test_truncateWithHashPadding(t *testing.T) {
	tests := map[string]struct {
		inString  string
		inMax     int
		inPadding int
		expected  string
	}{
		"less than max": {
			inString:  "mockString",
			inMax:     64,
			inPadding: 0,
			expected:  "mockString",
		},
		"truncate with hash padding": {
			inString:  "longapp-longenv-longsvc",
			inMax:     10,
			inPadding: 6,
			expected:  "longapp-lo7693be",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, truncateWithHashPadding(tc.inString, tc.inMax, tc.inPadding))
		})
	}
}
