// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func TestScheduledJob_ApplyEnv(t *testing.T) {
	testCases := map[string]struct {
		inputManifest *ScheduledJob
		inputEnv      string

		wantedManifest *ScheduledJob
		wantedErr      error
	}{
		"should return the same scheduled job if the environment does not exist": {
			inputManifest: newDefaultScheduledJob(),
			inputEnv:      "test",

			wantedManifest: newDefaultScheduledJob(),
		},
		"should preserve defaults and only override fields under 'environment'": {
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("report-generator"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("nginx"),
							},
						},
					},
					On: JobTriggerConfig{
						Schedule: aws.String("@hourly"),
					},
					JobFailureHandlerConfig: JobFailureHandlerConfig{
						Timeout: aws.String("5m"),
						Retries: aws.Int(1),
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod": {
						TaskConfig: TaskConfig{
							Variables: map[string]Variable{
								"LOG_LEVEL": {
									StringOrFromCFN{
										Plain: stringP("prod"),
									},
								},
							},
						},
					},
				},
			},
			inputEnv: "prod",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("report-generator"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("nginx"),
							},
						},
					},
					On: JobTriggerConfig{
						Schedule: aws.String("@hourly"),
					},
					JobFailureHandlerConfig: JobFailureHandlerConfig{
						Timeout: aws.String("5m"),
						Retries: aws.Int(1),
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
						},
						Variables: map[string]Variable{
							"LOG_LEVEL": {
								StringOrFromCFN{
									Plain: stringP("prod"),
								},
							},
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: PlacementArgOrString{
								PlacementString: placementStringP(PublicSubnetPlacement),
							},
						},
					},
				},
				Environments: nil,
			},
		},
		"with image build overridden by image location": {
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								ImageLocationOrBuild: ImageLocationOrBuild{
									Location: aws.String("env-override location"),
								},
							},
						},
					},
				},
			},
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
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
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							ImageLocationOrBuild: ImageLocationOrBuild{
								Location: aws.String("default location"),
							},
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
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
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
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
			actualManifest, actualErr := tc.inputManifest.applyEnv(tc.inputEnv)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, actualErr, tc.wantedErr.Error())
			} else {
				require.NoError(t, actualErr)
				require.Equal(t, tc.wantedManifest, actualManifest)
			}
		})
	}
}

func TestScheduledJob_RequiredEnvironmentFeatures(t *testing.T) {
	testCases := map[string]struct {
		mft    func(svc *ScheduledJob)
		wanted []string
	}{
		"no feature required by default": {
			mft: func(svc *ScheduledJob) {},
		},
		"nat feature required": {
			mft: func(svc *ScheduledJob) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: PlacementArgOrString{
							PlacementString: placementStringP(PrivateSubnetPlacement),
						},
					},
				}
			},
			wanted: []string{template.NATFeatureName},
		},
		"efs feature required by enabling managed volume": {
			mft: func(svc *ScheduledJob) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-managed-volume-1": {
							EFS: EFSConfigOrBool{
								Enabled: aws.Bool(true),
							},
						},
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
								},
							},
						},
					},
				}
			},
			wanted: []string{template.EFSFeatureName},
		},
		"efs feature not required because storage is imported": {
			mft: func(svc *ScheduledJob) {
				svc.Storage = Storage{
					Volumes: map[string]*Volume{
						"mock-imported-volume": {
							EFS: EFSConfigOrBool{
								Advanced: EFSVolumeConfiguration{
									FileSystemID: StringOrFromCFN{Plain: aws.String("fs-12345")},
								},
							},
						},
					},
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			inSvc := ScheduledJob{
				Workload: Workload{
					Name: aws.String("mock-svc"),
					Type: aws.String(manifestinfo.ScheduledJobType),
				},
			}
			tc.mft(&inSvc)
			got := inSvc.requiredEnvironmentFeatures()
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestScheduledJob_Publish(t *testing.T) {
	testCases := map[string]struct {
		mft *ScheduledJob

		wantedTopics []Topic
	}{
		"returns nil if there are no topics set": {
			mft: &ScheduledJob{},
		},
		"returns the list of topics if manifest publishes notifications": {
			mft: &ScheduledJob{
				ScheduledJobConfig: ScheduledJobConfig{
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
