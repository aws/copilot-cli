//go:build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTemplate_ParseScheduledJob(t *testing.T) {
	customResources := map[string]template.S3ObjectLocation{
		"EnvControllerFunction": {
			Bucket: "my-bucket",
			Key:    "key",
		},
	}

	testCases := map[string]struct {
		opts template.WorkloadOpts
	}{
		"renders a valid template by default": {
			opts: template.WorkloadOpts{
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders with timeout and no retries": {
			opts: template.WorkloadOpts{
				StateMachine: &template.StateMachineOpts{
					Timeout: aws.Int(3600),
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders with options": {
			opts: template.WorkloadOpts{
				StateMachine: &template.StateMachineOpts{
					Retries: aws.Int(5),
					Timeout: aws.Int(3600),
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders with options and addons": {
			opts: template.WorkloadOpts{
				StateMachine: &template.StateMachineOpts{
					Retries: aws.Int(3),
				},
				NestedStack: &template.WorkloadNestedStackOpts{
					StackName:       "AddonsStack",
					VariableOutputs: []string{"TableName"},
					SecretOutputs:   []string{"TablePassword"},
					PolicyOutputs:   []string{"TablePolicy"},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders with Windows platform": {
			opts: template.WorkloadOpts{
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				Platform: template.RuntimePlatformOpts{
					OS:   "windows",
					Arch: "x86_64",
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sess, err := sessions.ImmutableProvider().Default()
			require.NoError(t, err)
			cfn := cloudformation.New(sess)
			tpl := template.New()

			// WHEN
			content, err := tpl.ParseScheduledJob(tc.opts)
			require.NoError(t, err)

			// THEN
			_, err = cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
				TemplateBody: aws.String(content.String()),
			})
			require.NoError(t, err, content.String())
		})
	}
}

func TestTemplate_ParseLoadBalancedWebService(t *testing.T) {
	defaultHttpHealthCheck := template.HTTPHealthCheckOpts{
		HealthCheckPath: "/",
	}
	fakeS3Object := template.S3ObjectLocation{
		Bucket: "my-bucket",
		Key:    "key",
	}
	customResources := map[string]template.S3ObjectLocation{
		"DynamicDesiredCountFunction": fakeS3Object,
		"EnvControllerFunction":       fakeS3Object,
		"RulePriorityFunction":        fakeS3Object,
		"NLBCustomDomainFunction":     fakeS3Object,
		"NLBCertValidatorFunction":    fakeS3Object,
	}

	testCases := map[string]struct {
		opts template.WorkloadOpts
	}{
		"renders a valid template by default": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid grpc template by default": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with addons with no outputs": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				NestedStack: &template.WorkloadNestedStackOpts{
					StackName: "AddonsStack",
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				ALBEnabled:               true,
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders a valid template with addons with outputs": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				NestedStack: &template.WorkloadNestedStackOpts{
					StackName:       "AddonsStack",
					VariableOutputs: []string{"TableName"},
					SecretOutputs:   []string{"TablePassword"},
					PolicyOutputs:   []string{"TablePolicy"},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				ALBEnabled:               true,
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders a valid template with private subnet placement": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.DisablePublicIP,
					SubnetsType:    template.PrivateSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				ALBEnabled:               true,
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
		"renders a valid template with all storage options": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				Storage: &template.StorageOpts{
					Ephemeral: aws.Int(500),
					EFSPerms: []*template.EFSPermission{
						{
							AccessPointID: aws.String("ap-1234"),
							FilesystemID:  template.PlainFileSystemID("fs-5678"),
							Write:         true,
						},
					},
					MountPoints: []*template.MountPoint{
						{
							SourceVolume:  aws.String("efs"),
							ContainerPath: aws.String("/var/www"),
							ReadOnly:      aws.Bool(false),
						},
					},
					Volumes: []*template.Volume{
						{
							EFS: &template.EFSVolumeConfiguration{
								AccessPointID: aws.String("ap-1234"),
								Filesystem:    template.PlainFileSystemID("fs-5678"),
								IAM:           aws.String("ENABLED"),
								RootDirectory: aws.String("/"),
							},
							Name: aws.String("efs"),
						},
					},
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with minimal storage options": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				Storage: &template.StorageOpts{
					EFSPerms: []*template.EFSPermission{
						{
							FilesystemID: template.PlainFileSystemID("fs-5678"),
						},
					},
					MountPoints: []*template.MountPoint{
						{
							SourceVolume:  aws.String("efs"),
							ContainerPath: aws.String("/var/www"),
							ReadOnly:      aws.Bool(true),
						},
					},
					Volumes: []*template.Volume{
						{
							Name: aws.String("efs"),
							EFS: &template.EFSVolumeConfiguration{
								Filesystem:    template.PlainFileSystemID("fs-5678"),
								RootDirectory: aws.String("/"),
							},
						},
					},
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with ephemeral storage": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				Storage: &template.StorageOpts{
					Ephemeral: aws.Int(500),
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with entrypoint and command overrides": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				EntryPoint:               []string{"/bin/echo", "hello"},
				Command:                  []string{"world"},
				ServiceDiscoveryEndpoint: "test.app.local",
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with additional addons parameters": {
			opts: template.WorkloadOpts{
				ServiceDiscoveryEndpoint: "test.app.local",
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				AddonsExtraParams: `ServiceName: !Ref Service
DiscoveryServiceArn:
  Fn::GetAtt: [DiscoveryService, Arn]
`,
				ALBEnabled:      true,
				CustomResources: customResources,
				EnvVersion:      "v1.42.0",
				Version:         "v1.28.0",
			},
		},
		"renders a valid template with Windows platform": {
			opts: template.WorkloadOpts{
				ALBListener: &template.ALBListener{
					Rules: []template.ALBListenerRule{
						{
							Path:            "/",
							TargetPort:      "8080",
							TargetContainer: "main",
							HTTPVersion:     "GRPC",
							HTTPHealthCheck: defaultHttpHealthCheck,
							Stickiness:      "false",
						},
					},
				},
				Network: template.NetworkOpts{
					AssignPublicIP: template.EnablePublicIP,
					SubnetsType:    template.PublicSubnetsPlacement,
				},
				Platform: template.RuntimePlatformOpts{
					OS:   "windows",
					Arch: "x86_64",
				},
				ServiceDiscoveryEndpoint: "test.app.local",
				ALBEnabled:               true,
				CustomResources:          customResources,
				EnvVersion:               "v1.42.0",
				Version:                  "v1.28.0",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sess, err := sessions.ImmutableProvider().Default()
			require.NoError(t, err)
			cfn := cloudformation.New(sess)
			tpl := template.New()

			// WHEN
			content, err := tpl.ParseLoadBalancedWebService(tc.opts)
			require.NoError(t, err)

			// THEN
			_, err = cfn.ValidateTemplate(&cloudformation.ValidateTemplateInput{
				TemplateBody: aws.String(content.String()),
			})
			require.NoError(t, err, content.String())
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
		input template.NetworkOpts

		wantedNetworkConfig string
	}{
		"should render AWS VPC configuration for private subnets": {
			input: template.NetworkOpts{
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
			input: template.NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
				SecurityGroups: []template.SecurityGroup{
					template.PlainSecurityGroup("sg-1bcf1d5b"),
					template.PlainSecurityGroup("sg-asdasdas"),
					template.ImportedSecurityGroup("mydb-sg001"),
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
     - Fn::ImportValue: mydb-sg001
`,
		},
		"should render AWS VPC configuration without default environment security group": {
			input: template.NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
				SecurityGroups: []template.SecurityGroup{
					template.PlainSecurityGroup("sg-1bcf1d5b"),
					template.PlainSecurityGroup("sg-asdasdas"),
				},
				DenyDefaultSecurityGroup: true,
			},
			wantedNetworkConfig: `
 AwsvpcConfiguration:
   AssignPublicIp: DISABLED
   Subnets:
     Fn::Split:
       - ','
       - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
   SecurityGroups:
     - "sg-1bcf1d5b"
     - "sg-asdasdas"
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := template.New()
			wanted := make(map[interface{}]interface{})
			err := yaml.Unmarshal([]byte(tc.wantedNetworkConfig), &wanted)
			require.NoError(t, err, "unmarshal wanted config")

			// WHEN
			content, err := tpl.ParseLoadBalancedWebService(template.WorkloadOpts{
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
