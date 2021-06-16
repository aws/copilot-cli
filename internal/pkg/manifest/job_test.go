// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to create Manifest files.
package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestScheduledJob_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps ScheduledJobProps

		wantedTestData string
	}{
		"without timeout or retries": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:  "cuteness-aggregator",
					Image: "copilot/cuteness-aggregator",
				},
				Schedule: "@weekly",
			},
			wantedTestData: "scheduled-job-no-timeout-or-retries.yml",
		},
		"fully specified using cron schedule": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Schedule: "0 */2 * * *",
				Retries:  3,
				Timeout:  "1h30m",
			},
			wantedTestData: "scheduled-job-fully-specified.yml",
		},
		"with timeout and no retries": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Schedule: "@every 5h",
				Retries:  0,
				Timeout:  "3h",
			},
			wantedTestData: "scheduled-job-no-retries.yml",
		},
		"with retries and no timeout": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Schedule: "@every 5h",
				Retries:  5,
			},
			wantedTestData: "scheduled-job-no-timeout.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestData)
			wantedBytes, err := ioutil.ReadFile(path)
			require.NoError(t, err)
			manifest := NewScheduledJob(&tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

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
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("nginx"),
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
					Network: &NetworkConfig{
						VPC: &vpcConfig{
							Placement: stringP(PublicSubnetPlacement),
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod": {
						TaskConfig: TaskConfig{
							Variables: map[string]string{
								"LOG_LEVEL": "prod",
							},
						},
					},
				},
			},
			inputEnv: "prod",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("report-generator"),
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("nginx"),
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
						Variables: map[string]string{
							"LOG_LEVEL": "prod",
						},
					},
					Network: &NetworkConfig{
						VPC: &vpcConfig{
							Placement: stringP(PublicSubnetPlacement),
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
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
		},
		"with image location overridden by image location": {
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("default location"),
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								Location: aws.String("env-override location"),
							},
						},
					},
				},
			},
			inputEnv: "prod-iad",

			wantedManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
		},
		"with image build overridden by image build": {
			inputManifest: &ScheduledJob{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./Dockerfile"),
								},
							},
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
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
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("overridden build string"),
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
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("default location"),
						},
					},
				},
				Environments: map[string]*ScheduledJobConfig{
					"prod-iad": {
						ImageConfig: ImageWithHealthcheck{
							Image: Image{
								Build: BuildArgsOrString{
									BuildString: aws.String("overridden build string"),
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
					Type: aws.String(ScheduledJobType),
				},
				ScheduledJobConfig: ScheduledJobConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("overridden build string"),
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
			actualManifest, actualErr := tc.inputManifest.ApplyEnv(tc.inputEnv)

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
