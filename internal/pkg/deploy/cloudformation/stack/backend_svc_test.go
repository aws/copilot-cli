// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/template/templatetest"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// Test settings for container healthchecks in the backend service manifest.
var (
	testInterval    = 5 * time.Second
	testRetries     = 3
	testTimeout     = 10 * time.Second
	testStartPeriod = 0 * time.Second

	testServiceName = "frontend"
	testDockerfile  = "./frontend/Dockerfile"
)

func TestBackendService_Template(t *testing.T) {
	t.Cleanup(func() {
		fs = realEmbedFS
	})
	fs = templatetest.Stub{}

	t.Run("returns a wrapped error when addons template parsing fails", func(t *testing.T) {
		// GIVEN
		svc, err := NewBackendService(BackendServiceConfig{
			App:         &config.Application{},
			EnvManifest: &manifest.Environment{},
			Manifest: &manifest.BackendService{
				Workload: manifest.Workload{
					Name: aws.String("api"),
				},
			},
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{tplErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, "generate addons template for api: some error")
	})

	t.Run("returns a wrapped error when addons parameter parsing fails", func(t *testing.T) {
		// GIVEN
		svc, err := NewBackendService(BackendServiceConfig{
			App:         &config.Application{},
			EnvManifest: &manifest.Environment{},
			Manifest: &manifest.BackendService{
				Workload: manifest.Workload{
					Name: aws.String("api"),
				},
			},
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{paramsErr: errors.New("some error")},
		})
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, "parse addons parameters for api: some error")
	})

	t.Run("returns an error when failed to parse sidecar's exposed port", func(t *testing.T) {
		// GIVEN
		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
		})
		mft.Sidecars = map[string]*manifest.SidecarConfig{
			"xray": {
				Port: aws.String("80/80/80"),
			},
		}
		svc, err := NewBackendService(BackendServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           mft,
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{},
		})
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, "parse exposed ports in service manifest api: cannot parse port mapping from 80/80/80")
	})

	t.Run("returns an error when failed to convert sidecar configuration", func(t *testing.T) {
		// GIVEN
		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
		})
		mft.Sidecars = map[string]*manifest.SidecarConfig{
			"xray": {
				Port: aws.String("80"),
				ImageOverride: manifest.ImageOverride{
					Command: manifest.CommandOverride{
						String: aws.String("[bad'command]"),
					},
				},
			},
		}
		svc, err := NewBackendService(BackendServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           mft,
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{},
		})
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, `convert the sidecar configuration for service api: convert "command" to string slice: convert string into tokens using shell-style rules: EOF found when expecting closing quote`)
	})

	t.Run("returns an error when failed to parse autoscaling template", func(t *testing.T) {
		// GIVEN
		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
		})
		badRange := manifest.IntRangeBand("badRange")
		mft.Count.AdvancedCount = manifest.AdvancedCount{
			Range: manifest.Range{
				Value: &badRange,
			},
		}
		svc, err := NewBackendService(BackendServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           mft,
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{},
		})
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, "convert the advanced count configuration for service api: invalid range value badRange. Should be in format of ${min}-${max}")
	})

	t.Run("returns wrapped error when failed to parse the template", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		parser := mocks.NewMockbackendSvcReadParser(ctrl)
		parser.EXPECT().ParseBackendService(gomock.Any()).Return(nil, errors.New("some error"))

		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
		})
		svc, err := NewBackendService(BackendServiceConfig{
			App:                &config.Application{},
			EnvManifest:        &manifest.Environment{},
			Manifest:           mft,
			ArtifactBucketName: "mockBucket",
			Addons:             mockAddons{},
		})
		svc.parser = parser
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.EqualError(t, err, "parse backend service template: some error")
	})

	t.Run("renders template without a load balancer", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
			HealthCheck: manifest.ContainerHealthCheck{
				Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				Interval:    &testInterval,
				Retries:     &testRetries,
				Timeout:     &testTimeout,
				StartPeriod: &testStartPeriod,
			},
		})
		mft.EntryPoint = manifest.EntryPointOverride{
			String:      nil,
			StringSlice: []string{"enter", "from"},
		}
		mft.Command = manifest.CommandOverride{
			String:      nil,
			StringSlice: []string{"here"},
		}
		mft.ExecuteCommand = manifest.ExecuteCommand{Enable: aws.Bool(true)}
		mft.DeployConfig = manifest.DeploymentConfig{
			DeploymentControllerConfig: manifest.DeploymentControllerConfig{
				Rolling: aws.String("recreate"),
			}}
		privatePlacement := manifest.PrivateSubnetPlacement
		mft.Network.Connect.Alias = aws.String("api")
		mft.Network.Connect.EnableServiceConnect = aws.Bool(true)
		mft.Network.VPC.Placement = manifest.PlacementArgOrString{
			PlacementString: &privatePlacement,
		}
		mft.Network.VPC.SecurityGroups = manifest.SecurityGroupsIDsOrConfig{}

		var actual template.WorkloadOpts
		parser := mocks.NewMockbackendSvcReadParser(ctrl)
		parser.EXPECT().ParseBackendService(gomock.Any()).DoAndReturn(func(in template.WorkloadOpts) (*template.Content, error) {
			actual = in // Capture the translated object.
			return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
		})
		addons := mockAddons{
			tpl: `
Resources:
  MyTable:
    Type: AWS::DynamoDB::Table
Outputs:
  MyTable:
    Value: !Ref MyTable`,
			params: "",
		}

		svc, err := NewBackendService(BackendServiceConfig{
			App: &config.Application{
				Name: "phonetool",
			},
			EnvManifest: &manifest.Environment{
				Workload: manifest.Workload{
					Name: aws.String("test"),
				},
			},
			ArtifactBucketName: "mockBucket",
			Manifest:           mft,
			RuntimeConfig: RuntimeConfig{
				PushedImages: map[string]ECRImage{
					"test": {
						RepoURL:  testImageRepoURL,
						ImageTag: testImageTag,
					},
				},
				CustomResourcesURL: map[string]string{
					"EnvControllerFunction":       "https://my-bucket.s3.Region.amazonaws.com/sha1/envcontroller.zip",
					"DynamicDesiredCountFunction": "https://my-bucket.s3.Region.amazonaws.com/sha2/count.zip",
				},
			},
			Addons: addons,
		})
		svc.parser = parser
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, template.WorkloadOpts{
			AppName:      "phonetool",
			EnvName:      "test",
			WorkloadName: "api",
			WorkloadType: manifestinfo.BackendServiceType,
			HealthCheck: &template.ContainerHealthCheck{
				Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				Interval:    aws.Int64(5),
				Retries:     aws.Int64(3),
				StartPeriod: aws.Int64(0),
				Timeout:     aws.Int64(10),
			},
			ServiceConnectOpts: template.ServiceConnectOpts{
				Server: &template.ServiceConnectServer{
					Name:  "api",
					Port:  "8080",
					Alias: "api",
				},
				Client: true,
			},
			GracePeriod: aws.Int64(manifest.DefaultHealthCheckGracePeriod),
			CustomResources: map[string]template.S3ObjectLocation{
				"EnvControllerFunction": {
					Bucket: "my-bucket",
					Key:    "sha1/envcontroller.zip",
				},
				"DynamicDesiredCountFunction": {
					Bucket: "my-bucket",
					Key:    "sha2/count.zip",
				},
			},
			ExecuteCommand: &template.ExecuteCommandOpts{},
			NestedStack: &template.WorkloadNestedStackOpts{
				StackName:       addon.StackName,
				VariableOutputs: []string{"MyTable"},
			},
			Network: template.NetworkOpts{
				AssignPublicIP: template.DisablePublicIP,
				SubnetsType:    template.PrivateSubnetsPlacement,
				SecurityGroups: []template.SecurityGroup{},
			},
			DeploymentConfiguration: template.DeploymentConfigurationOpts{
				MinHealthyPercent: 0,
				MaxPercent:        100,
			},
			EntryPoint: []string{"enter", "from"},
			Command:    []string{"here"},
			PortMappings: []*template.PortMapping{
				{
					Protocol:      "tcp",
					ContainerName: "api",
					ContainerPort: 8080,
				},
			},
		}, actual)
	})

	t.Run("renders template with internal load balancer", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mft := manifest.NewBackendService(manifest.BackendServiceProps{
			WorkloadProps: manifest.WorkloadProps{
				Name:       "api",
				Dockerfile: testDockerfile,
			},
			Port: 8080,
			HealthCheck: manifest.ContainerHealthCheck{
				Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				Interval:    &testInterval,
				Retries:     &testRetries,
				Timeout:     &testTimeout,
				StartPeriod: &testStartPeriod,
			},
		})
		mft.EntryPoint = manifest.EntryPointOverride{
			String:      nil,
			StringSlice: []string{"enter", "from"},
		}
		mft.Command = manifest.CommandOverride{
			String:      nil,
			StringSlice: []string{"here"},
		}
		mft.ExecuteCommand = manifest.ExecuteCommand{Enable: aws.Bool(true)}
		mft.DeployConfig = manifest.DeploymentConfig{
			DeploymentControllerConfig: manifest.DeploymentControllerConfig{
				Rolling: aws.String("recreate"),
			}}
		mft.HTTP = manifest.HTTP{
			Main: manifest.RoutingRule{
				Path: aws.String("/albPath"),
				HealthCheck: manifest.HealthCheckArgsOrString{
					Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
						Path:               aws.String("/healthz"),
						Port:               aws.Int(4200),
						SuccessCodes:       aws.String("418"),
						HealthyThreshold:   aws.Int64(64),
						UnhealthyThreshold: aws.Int64(63),
						Timeout:            (*time.Duration)(aws.Int64(int64(62 * time.Second))),
						Interval:           (*time.Duration)(aws.Int64(int64(61 * time.Second))),
						GracePeriod:        (*time.Duration)(aws.Int64(int64(1 * time.Minute))),
					}),
				},
				Stickiness:          aws.Bool(true),
				DeregistrationDelay: (*time.Duration)(aws.Int64(int64(59 * time.Second))),
				AllowedSourceIps:    []manifest.IPNet{"10.0.1.0/24"},
				TargetContainer:     aws.String("envoy"),
			},
		}
		mft.Sidecars = map[string]*manifest.SidecarConfig{
			"envoy": {
				Image: manifest.Union[*string, manifest.ImageLocationOrBuild]{
					Advanced: manifest.ImageLocationOrBuild{
						Build: manifest.BuildArgsOrString{
							BuildString: aws.String("./Dockerfile"),
						},
					},
				},
				Port: aws.String("443"),
			},
			"nginx": {
				Image: manifest.Union[*string, manifest.ImageLocationOrBuild]{
					Basic: aws.String("mockImageURL"),
				},
			},
		}
		privatePlacement := manifest.PrivateSubnetPlacement
		mft.Network.Connect.EnableServiceConnect = aws.Bool(true)
		mft.Network.VPC.Placement = manifest.PlacementArgOrString{
			PlacementString: &privatePlacement,
		}
		mft.Network.VPC.SecurityGroups = manifest.SecurityGroupsIDsOrConfig{}

		var actual template.WorkloadOpts
		parser := mocks.NewMockbackendSvcReadParser(ctrl)
		parser.EXPECT().ParseBackendService(gomock.Any()).DoAndReturn(func(in template.WorkloadOpts) (*template.Content, error) {
			actual = in // Capture the translated object.
			return &template.Content{Buffer: bytes.NewBufferString("template")}, nil
		})
		addons := mockAddons{
			tpl: `
Resources:
  MyTable:
    Type: AWS::DynamoDB::Table
Outputs:
  MyTable:
    Value: !Ref MyTable`,
			params: "",
		}

		svc, err := NewBackendService(BackendServiceConfig{
			App: &config.Application{
				Name: "phonetool",
			},
			EnvManifest: &manifest.Environment{
				Workload: manifest.Workload{
					Name: aws.String("test"),
				},
			},
			Manifest: mft,
			RuntimeConfig: RuntimeConfig{
				PushedImages: map[string]ECRImage{
					"test": {
						RepoURL:           testImageRepoURL,
						ImageTag:          testImageTag,
						MainContainerName: "test",
						ContainerName:     "test",
					},
					"envoy": {
						RepoURL:           testImageRepoURL,
						ImageTag:          testImageTag,
						MainContainerName: "test",
						ContainerName:     "envoy",
					},
				},
			},
			Addons:             addons,
			ArtifactBucketName: "my-bucket",
		})
		svc.parser = parser
		require.NoError(t, err)

		// WHEN
		_, err = svc.Template()

		// THEN
		require.NoError(t, err)
		require.Equal(t, template.WorkloadOpts{
			AppName:      "phonetool",
			EnvName:      "test",
			WorkloadName: "api",
			WorkloadType: manifestinfo.BackendServiceType,
			HealthCheck: &template.ContainerHealthCheck{
				Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
				Interval:    aws.Int64(5),
				Retries:     aws.Int64(3),
				StartPeriod: aws.Int64(0),
				Timeout:     aws.Int64(10),
			},
			Sidecars: []*template.SidecarOpts{
				{
					Name:  "envoy",
					Image: aws.String(fmt.Sprintf("%s:%s-%s", testImageRepoURL, "envoy", testImageTag)),
					PortMappings: []*template.PortMapping{
						{
							Protocol:      "tcp",
							ContainerName: "envoy",
							ContainerPort: 443,
						},
					},
				},
				{
					Name:         "nginx",
					Image:        aws.String("mockImageURL"),
					PortMappings: []*template.PortMapping{},
				},
			},
			ServiceConnectOpts: template.ServiceConnectOpts{
				Server: &template.ServiceConnectServer{
					Name: "envoy",
					Port: "443",
				},
				Client: true,
			},
			CustomResources: map[string]template.S3ObjectLocation{
				"EnvControllerFunction": {
					Bucket: "my-bucket",
					Key:    "manual/scripts/custom-resources/envcontrollerfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"DynamicDesiredCountFunction": {
					Bucket: "my-bucket",
					Key:    "manual/scripts/custom-resources/dynamicdesiredcountfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
				"RulePriorityFunction": {
					Bucket: "my-bucket",
					Key:    "manual/scripts/custom-resources/rulepriorityfunction/8932747ba5dbff619d89b92d0033ef1d04f7dd1b055e073254907d4e38e3976d.zip",
				},
			},
			ExecuteCommand: &template.ExecuteCommandOpts{},
			NestedStack: &template.WorkloadNestedStackOpts{
				StackName:       addon.StackName,
				VariableOutputs: []string{"MyTable"},
			},
			Network: template.NetworkOpts{
				AssignPublicIP: template.DisablePublicIP,
				SubnetsType:    template.PrivateSubnetsPlacement,
				SecurityGroups: []template.SecurityGroup{},
			},
			DeploymentConfiguration: template.DeploymentConfigurationOpts{
				MinHealthyPercent: 0,
				MaxPercent:        100,
			},
			EntryPoint:  []string{"enter", "from"},
			Command:     []string{"here"},
			GracePeriod: aws.Int64(60),
			ALBListener: &template.ALBListener{
				Rules: []template.ALBListenerRule{
					{
						Path:            "/albPath",
						TargetContainer: "envoy",
						TargetPort:      "443",
						HTTPHealthCheck: template.HTTPHealthCheckOpts{
							HealthCheckPath:    "/healthz",
							GracePeriod:        60,
							Port:               "4200",
							SuccessCodes:       "418",
							HealthyThreshold:   aws.Int64(64),
							UnhealthyThreshold: aws.Int64(63),
							Interval:           aws.Int64(61),
							Timeout:            aws.Int64(62),
						},
						DeregistrationDelay: aws.Int64(int64(59)),
						AllowedSourceIps: []string{
							"10.0.1.0/24",
						},
						Stickiness: "true",
					},
				},
				HostedZoneAliases: template.AliasesForHostedZone{},
				MainContainerPort: "8080",
			},
			ALBEnabled: true,
			PortMappings: []*template.PortMapping{
				{
					Protocol:      "tcp",
					ContainerName: "api",
					ContainerPort: 8080,
				},
			},
		}, actual)
	})
}

func TestBackendService_Parameters(t *testing.T) {
	testBackendSvcManifest := manifest.NewBackendService(manifest.BackendServiceProps{
		WorkloadProps: manifest.WorkloadProps{
			Name:       testServiceName,
			Dockerfile: testDockerfile,
		},
		Port: 8080,
		HealthCheck: manifest.ContainerHealthCheck{
			Command:     []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"},
			Interval:    &testInterval,
			Retries:     &testRetries,
			Timeout:     &testTimeout,
			StartPeriod: &testStartPeriod,
		},
	})

	conf := &BackendService{
		ecsWkld: &ecsWkld{
			wkld: &wkld{
				name: aws.StringValue(testBackendSvcManifest.Name),
				env:  testEnvName,
				app:  testAppName,
				image: manifest.Image{
					ImageLocationOrBuild: manifest.ImageLocationOrBuild{
						Location: aws.String("mockLocation"),
					},
				},
			},
			tc: testBackendSvcManifest.BackendServiceConfig.TaskConfig,
		},
		manifest: testBackendSvcManifest,
	}

	// WHEN
	params, _ := conf.Parameters()

	// THEN
	require.ElementsMatch(t, []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String(WorkloadAppNameParamKey),
			ParameterValue: aws.String("phonetool"),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvNameParamKey),
			ParameterValue: aws.String("test"),
		},
		{
			ParameterKey:   aws.String(WorkloadNameParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerImageParamKey),
			ParameterValue: aws.String("mockLocation"),
		},
		{
			ParameterKey:   aws.String(WorkloadContainerPortParamKey),
			ParameterValue: aws.String("8080"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCPUParamKey),
			ParameterValue: aws.String("256"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskMemoryParamKey),
			ParameterValue: aws.String("512"),
		},
		{
			ParameterKey:   aws.String(WorkloadTaskCountParamKey),
			ParameterValue: aws.String("1"),
		},
		{
			ParameterKey:   aws.String(WorkloadLogRetentionParamKey),
			ParameterValue: aws.String("30"),
		},
		{
			ParameterKey:   aws.String(WorkloadAddonsTemplateURLParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadEnvFileARNParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadArtifactKeyARNParamKey),
			ParameterValue: aws.String(""),
		},
		{
			ParameterKey:   aws.String(WorkloadTargetContainerParamKey),
			ParameterValue: aws.String("frontend"),
		},
		{
			ParameterKey:   aws.String(WorkloadTargetPortParamKey),
			ParameterValue: aws.String("8080"),
		},
	}, params)
}
