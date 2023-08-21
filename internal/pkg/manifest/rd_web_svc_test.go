// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewRequestDrivenWebService(t *testing.T) {
	testCases := map[string]struct {
		input *RequestDrivenWebServiceProps

		wantedStruct *RequestDrivenWebService
	}{
		"should return an instance of RequestDrivenWebService": {
			input: &RequestDrivenWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "frontend",
					Dockerfile: "./Dockerfile",
				},
				Port: uint16(80),
			},

			wantedStruct: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("frontend"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
						},
						Port: aws.Uint16(80),
					},
					InstanceConfig: AppRunnerInstanceConfig{
						CPU:    aws.Int(1024),
						Memory: aws.Int(2048),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			svc := NewRequestDrivenWebService(tc.input)

			require.Equal(t, tc.wantedStruct.Name, svc.Name)
			require.Equal(t, tc.wantedStruct.Type, svc.Type)
			require.Equal(t, tc.wantedStruct.Environments, svc.Environments)
			require.Equal(t, tc.wantedStruct.InstanceConfig, svc.InstanceConfig)
			require.Equal(t, tc.wantedStruct.ImageConfig, svc.ImageConfig)
			require.Equal(t, tc.wantedStruct.Tags, svc.Tags)
			require.Equal(t, tc.wantedStruct.Variables, svc.Variables)

		})
	}
}

func TestRequestDrivenWebService_UnmarshalYaml(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct RequestDrivenWebService
		wantedError  error
	}{
		"should unmarshal basic yaml configuration": {
			inContent: []byte(
				"name: test-service\n" +
					"type: Request-Driven Web Service\n" +
					"cpu: 512\n" +
					"memory: 1024\n" +
					"image:\n" +
					"  build: ./Dockerfile\n" +
					"  port: 80\n",
			),

			wantedStruct: RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("test-service"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildString: aws.String("./Dockerfile"),
								},
							},
						},
						Port: aws.Uint16(80),
					},
					InstanceConfig: AppRunnerInstanceConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(1024),
					},
				},
			},
		},
		"should unmarshal image location": {
			inContent: []byte(
				"image:\n" +
					"  location: test-repository/image@digest\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("test-repository/image@digest"),
							},
						},
					},
				},
			},
		},
		"should unmarshal image build configuration": {
			inContent: []byte(
				"image:\n" +
					"  build:\n" +
					"    dockerfile: ./Dockerfile\n" +
					"    context: context/dir\n" +
					"    target: build-stage\n" +
					"    cache_from:\n" +
					"      - image:tag\n" +
					"    args:\n" +
					"      a: 1\n" +
					"      b: 2\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Context:    aws.String("context/dir"),
										Dockerfile: aws.String("./Dockerfile"),
										Target:     aws.String("build-stage"),
										CacheFrom:  []string{"image:tag"},
										Args:       map[string]string{"a": "1", "b": "2"},
									},
								},
							},
						},
					},
				},
			},
		},
		"should unmarshal environment variables": {
			inContent: []byte(
				"variables:\n" +
					"  LOG_LEVEL: info\n" +
					"  NODE_ENV: development\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					Variables: map[string]Variable{
						"LOG_LEVEL": {
							StringOrFromCFN{
								Plain: stringP("info"),
							},
						},
						"NODE_ENV": {
							StringOrFromCFN{
								Plain: stringP("development"),
							},
						},
					},
				},
			},
		},
		"should unmarshal tags": {
			inContent: []byte(
				"tags:\n" +
					"  owner: account-id\n" +
					"  project: my-project\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					Tags: map[string]string{
						"owner":   "account-id",
						"project": "my-project",
					},
				},
			},
		},
		"should unmarshal http configuration": {
			inContent: []byte(
				"http:\n" +
					"  healthcheck:\n" +
					"    path: /healthcheck\n" +
					"    healthy_threshold: 3\n" +
					"    unhealthy_threshold: 5\n" +
					"    interval: 10s\n" +
					"    timeout: 5s\n" +
					"  alias: convex.domain.com",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					RequestDrivenWebServiceHttpConfig: RequestDrivenWebServiceHttpConfig{
						HealthCheckConfiguration: HealthCheckArgsOrString{
							Union: AdvancedToUnion[string](HTTPHealthCheckArgs{
								Path:               aws.String("/healthcheck"),
								HealthyThreshold:   aws.Int64(3),
								UnhealthyThreshold: aws.Int64(5),
								Interval:           durationp(10 * time.Second),
								Timeout:            durationp(5 * time.Second),
							}),
						},
						Alias: aws.String("convex.domain.com"),
					},
				},
			},
		},
		"should unmarshal healthcheck shorthand": {
			inContent: []byte(
				"http:\n" +
					"  healthcheck: /healthcheck\n",
			),

			wantedStruct: RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					RequestDrivenWebServiceHttpConfig: RequestDrivenWebServiceHttpConfig{
						HealthCheckConfiguration: HealthCheckArgsOrString{
							Union: BasicToUnion[string, HTTPHealthCheckArgs]("/healthcheck"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var svc RequestDrivenWebService
			err := yaml.Unmarshal(tc.inContent, &svc)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct, svc)
			}
		})
	}
}

func TestRequestDrivenWebService_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inManifest *RequestDrivenWebService

		wantedBinary []byte
		wantedError  error
	}{
		"error parsing template": {
			inManifest: &RequestDrivenWebService{},

			wantedError: errors.New("test error"),
		},
		"returns rendered content": {
			inManifest: &RequestDrivenWebService{},

			wantedBinary: []byte("test content"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockParser := mocks.NewMockParser(ctrl)
			tc.inManifest.parser = mockParser
			var wantedTemplContent *template.Content = nil

			if tc.wantedBinary != nil {
				wantedTemplContent = &template.Content{Buffer: bytes.NewBufferString(string(tc.wantedBinary))}
			}

			mockParser.
				EXPECT().
				Parse(requestDrivenWebSvcManifestPath, *tc.inManifest, gomock.Any()).
				Return(wantedTemplContent, tc.wantedError)

			b, err := tc.inManifest.MarshalBinary()

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedBinary, b)
		})
	}
}

func TestRequestDrivenWebService_Port(t *testing.T) {
	// GIVEN
	mft := RequestDrivenWebService{
		RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
			ImageConfig: ImageWithPort{
				Port: uint16P(80),
			},
		},
	}

	// WHEN
	actual, ok := mft.Port()

	// THEN
	require.True(t, ok)
	require.Equal(t, uint16(80), actual)
}

func TestRequestDrivenWebService_ContainerPlatform(t *testing.T) {
	t.Run("should return platform string with values found in args", func(t *testing.T) {
		// GIVEN
		mft := RequestDrivenWebService{
			RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
				InstanceConfig: AppRunnerInstanceConfig{
					Platform: PlatformArgsOrString{
						PlatformArgs: PlatformArgs{
							OSFamily: aws.String("ososos"),
							Arch:     aws.String("arch"),
						},
					},
				},
			},
		}
		// WHEN
		actual := mft.ContainerPlatform()

		// THEN
		require.Equal(t, "ososos/arch", actual)
	})
	t.Run("should return default platform if platform field empty", func(t *testing.T) {
		// GIVEN
		mft := RequestDrivenWebService{
			RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
				InstanceConfig: AppRunnerInstanceConfig{
					Platform: PlatformArgsOrString{
						PlatformString: nil,
					},
				},
			},
		}
		// WHEN
		actual := mft.ContainerPlatform()

		// THEN
		require.Equal(t, "linux/amd64", actual)

	})
}

func TestRequestDrivenWebService_Publish(t *testing.T) {
	testCases := map[string]struct {
		mft *RequestDrivenWebService

		wantedTopics []Topic
	}{
		"returns nil if there are no topics set": {
			mft: &RequestDrivenWebService{},
		},
		"returns the list of topics if manifest publishes notifications": {
			mft: &RequestDrivenWebService{
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					PublishConfig: PublishConfig{
						Topics: []Topic{
							{
								Name: stringP("hello"),
							},
						},
					},
				},
			},
			wantedTopics: []Topic{
				{
					Name: stringP("hello"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual := tc.mft.Publish()

			// THEN
			require.Equal(t, tc.wantedTopics, actual)
		})
	}
}

func TestRequestDrivenWebService_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		in         *RequestDrivenWebService
		envToApply string

		wanted *RequestDrivenWebService
	}{
		"with image build overridden by image location": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
		"with image location overridden by image location": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
		},
		"with image build overridden by image build": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildArgs: DockerBuildArgs{
										Dockerfile: aws.String("./Dockerfile"),
									},
								},
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildString: aws.String("overridden build string"),
									},
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
		},
		"with image location overridden by image build": {
			in: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*RequestDrivenWebServiceConfig{
					"prod-iad": {
						ImageConfig: ImageWithPort{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Build: BuildArgsOrString{
										BuildString: aws.String("overridden build string"),
									},
								},
							},
						},
					},
				},
			},
			envToApply: "prod-iad",

			wanted: &RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
				RequestDrivenWebServiceConfig: RequestDrivenWebServiceConfig{
					ImageConfig: ImageWithPort{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			conf, _ := tc.in.applyEnv(tc.envToApply)

			// THEN
			require.Equal(t, tc.wanted, conf, "returned configuration should have overrides from the environment")
		})
	}
}

func TestRequestDrivenWebService_RequiredEnvironmentFeatures(t *testing.T) {
	testCases := map[string]struct {
		mft    func(svc *RequestDrivenWebService)
		wanted []string
	}{
		"no feature required by default": {
			mft: func(svc *RequestDrivenWebService) {},
		},
		"nat feature required": {
			mft: func(svc *RequestDrivenWebService) {
				svc.Network = RequestDrivenWebServiceNetworkConfig{
					VPC: rdwsVpcConfig{
						Placement: PlacementArgOrString{
							PlacementString: placementStringP(PrivateSubnetPlacement),
						},
					},
				}
			},
			wanted: []string{template.NATFeatureName},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			inSvc := RequestDrivenWebService{
				Workload: Workload{
					Name: aws.String("mock-svc"),
					Type: aws.String(manifestinfo.RequestDrivenWebServiceType),
				},
			}
			tc.mft(&inSvc)
			got := inSvc.requiredEnvironmentFeatures()
			require.Equal(t, tc.wanted, got)
		})
	}
}
