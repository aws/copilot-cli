//go:build localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancedWebService_InitialManifestIntegration(t *testing.T) {
	testCases := map[string]struct {
		inProps LoadBalancedWebServiceProps

		wantedTestdata string
	}{
		"default": {
			inProps: LoadBalancedWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "frontend",
					Dockerfile: "./frontend/Dockerfile",
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
				},
				Port: 80,
			},
			wantedTestdata: "lb-svc.yml",
		},
		"with placement private": {
			inProps: LoadBalancedWebServiceProps{
				WorkloadProps: &WorkloadProps{
					Name:       "frontend",
					Dockerfile: "./frontend/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"phonetool",
					},
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
				},
				Port: 80,
			},
			wantedTestdata: "lb-svc-placement-private.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := os.ReadFile(path)
			require.NoError(t, err)
			manifest := NewLoadBalancedWebService(&tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestBackendSvc_InitialManifestIntegration(t *testing.T) {
	testCases := map[string]struct {
		inProps BackendServiceProps

		wantedTestdata string
	}{
		"without healthcheck and port and with private only environments": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "subscribers",
					Dockerfile: "./subscribers/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"phonetool",
					},
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs: PlatformArgs{
						OSFamily: nil,
						Arch:     nil,
					},
				},
			},
			wantedTestdata: "backend-svc-nohealthcheck-placement.yml",
		},
		"with custom healthcheck command": {
			inProps: BackendServiceProps{
				WorkloadProps: WorkloadProps{
					Name:  "subscribers",
					Image: "flask-sample",
				},
				HealthCheck: ContainerHealthCheck{
					Command:     []string{"CMD-SHELL", "curl -f http://localhost:8080 || exit 1"},
					Interval:    durationp(6 * time.Second),
					Retries:     aws.Int(0),
					Timeout:     durationp(20 * time.Second),
					StartPeriod: durationp(15 * time.Second),
				},
				Port: 8080,
			},
			wantedTestdata: "backend-svc-customhealthcheck.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := os.ReadFile(path)
			require.NoError(t, err)
			manifest := NewBackendService(tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestWorkerSvc_InitialManifestIntegration(t *testing.T) {
	testCases := map[string]struct {
		inProps WorkerServiceProps

		wantedTestdata string
	}{
		"without subscribe and with private only environments": {
			inProps: WorkerServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "testers",
					Dockerfile: "./testers/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"phonetool",
					},
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs: PlatformArgs{
						OSFamily: nil,
						Arch:     nil,
					},
				},
			},
			wantedTestdata: "worker-svc-nosubscribe-placement.yml",
		},
		"with subscribe": {
			inProps: WorkerServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "testers",
					Dockerfile: "./testers/Dockerfile",
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs: PlatformArgs{
						OSFamily: nil,
						Arch:     nil,
					},
				},
				Topics: []TopicSubscription{
					{
						Name:    aws.String("testTopic"),
						Service: aws.String("service4TestTopic"),
					},
					{
						Name:    aws.String("testTopic2"),
						Service: aws.String("service4TestTopic2"),
					},
				},
			},
			wantedTestdata: "worker-svc-subscribe.yml",
		},
		"with fifo topic subscription with default fifo queue": {
			inProps: WorkerServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "testers",
					Dockerfile: "./testers/Dockerfile",
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs: PlatformArgs{
						OSFamily: nil,
						Arch:     nil,
					},
				},
				Topics: []TopicSubscription{
					{
						Name:    aws.String("testTopic.fifo"),
						Service: aws.String("service4TestTopic"),
					},
				},
			},
			wantedTestdata: "worker-svc-with-default-fifo-queue.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := os.ReadFile(path)
			require.NoError(t, err)
			manifest := NewWorkerService(tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestScheduledJob_InitialManifestIntegration(t *testing.T) {
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
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
				},
				Schedule: "@weekly",
			},
			wantedTestData: "scheduled-job-no-timeout-or-retries.yml",
		},
		"fully specified using cron schedule with placement set to private": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
					PrivateOnlyEnvironments: []string{
						"phonetool",
					},
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
				},
				Schedule: "0 */2 * * *",
				Retries:  3,
				Timeout:  "1h30m",
			},
			wantedTestData: "scheduled-job-fully-specified-placement.yml",
		},
		"with timeout and no retries": {
			inProps: ScheduledJobProps{
				WorkloadProps: &WorkloadProps{
					Name:       "cuteness-aggregator",
					Dockerfile: "./cuteness-aggregator/Dockerfile",
				},
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
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
				Platform: PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs:   PlatformArgs{},
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
			wantedBytes, err := os.ReadFile(path)
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

func TestEnvironment_InitialManifestIntegration(t *testing.T) {
	testCases := map[string]struct {
		inProps        EnvironmentProps
		wantedTestData string
	}{
		"fully configured with customized vpc resources": {
			inProps: EnvironmentProps{
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					VPCConfig: &config.AdjustVPC{
						CIDR:               "mock-cidr-0",
						AZs:                []string{"mock-az-1", "mock-az-2"},
						PublicSubnetCIDRs:  []string{"mock-cidr-1", "mock-cidr-2"},
						PrivateSubnetCIDRs: []string{"mock-cidr-3", "mock-cidr-4"},
					},
					ImportCertARNs:     []string{"mock-cert-1", "mock-cert-2"},
					InternalALBSubnets: []string{"mock-subnet-id-3", "mock-subnet-id-4"},
				},
				Telemetry: &config.Telemetry{
					EnableContainerInsights: false,
				},
			},
			wantedTestData: "environment-adjust-vpc.yml",
		},
		"fully configured with customized vpc resources including imported private subnets": {
			inProps: EnvironmentProps{
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportVPC: &config.ImportVPC{
						ID:               "mock-vpc-id",
						PrivateSubnetIDs: []string{"mock-subnet-id-3", "mock-subnet-id-4"},
					},
					ImportCertARNs:              []string{"mock-cert-1", "mock-cert-2"},
					InternalALBSubnets:          []string{"mock-subnet-id-3", "mock-subnet-id-4"},
					EnableInternalALBVPCIngress: false,
				},

				Telemetry: &config.Telemetry{
					EnableContainerInsights: false,
				},
			},
			wantedTestData: "environment-adjust-vpc-private-subnets.yml",
		},
		"fully configured with imported vpc resources": {
			inProps: EnvironmentProps{
				Name: "test",
				CustomConfig: &config.CustomizeEnv{
					ImportVPC: &config.ImportVPC{
						ID:               "mock-vpc-id",
						PublicSubnetIDs:  []string{"mock-subnet-id-1", "mock-subnet-id-2"},
						PrivateSubnetIDs: []string{"mock-subnet-id-3", "mock-subnet-id-4"},
					},
					ImportCertARNs: []string{"mock-cert-1", "mock-cert-2"},
				},
				Telemetry: &config.Telemetry{
					EnableContainerInsights: true,
				},
			},
			wantedTestData: "environment-import-vpc.yml",
		},
		"basic manifest": {
			inProps: EnvironmentProps{
				Name: "test",
			},
			wantedTestData: "environment-default.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestData)
			wantedBytes, err := os.ReadFile(path)
			require.NoError(t, err)
			manifest := NewEnvironment(&tc.inProps)

			// WHEN
			tpl, err := manifest.MarshalBinary()
			require.NoError(t, err)

			// THEN
			require.Equal(t, string(wantedBytes), string(tpl))
		})
	}
}

func TestPipelineManifest_InitialManifest_Integration(t *testing.T) {
	testCases := map[string]struct {
		inProvider Provider
		inStages   []PipelineStage

		wantedTestData string
		wantedError    error
	}{
		"basic pipeline manifest": {
			inProvider: &githubProvider{
				properties: &GitHubProperties{
					RepositoryURL: "mock-url",
					Branch:        "main",
				},
			},
			inStages: []PipelineStage{
				{
					Name: "test",
				},
				{
					Name: "prod",
				},
			},
			wantedTestData: "pipeline-basic.yml",
		},
		"environment pipeline manifest with template configurations": {
			inProvider: &githubProvider{
				properties: &GitHubProperties{
					RepositoryURL: "mock-url",
					Branch:        "main",
				},
			},
			inStages: []PipelineStage{
				{
					Name: "test",
					Deployments: Deployments{
						"deploy-env": &Deployment{
							TemplatePath:   "infrastructure/test.env.yml",
							TemplateConfig: "infrastructure/test.env.params.json",
							StackName:      "app-test",
						},
					},
				},
				{
					Name: "prod",
					Deployments: Deployments{
						"deploy-env": &Deployment{
							TemplatePath:   "infrastructure/prod.env.yml",
							TemplateConfig: "infrastructure/prod.env.params.json",
							StackName:      "app-prod",
						},
					},
				},
			},
			wantedTestData: "pipeline-environment.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			path := filepath.Join("testdata", tc.wantedTestData)
			wantedBytes, err := os.ReadFile(path)
			require.NoError(t, err)

			manifest, err := NewPipeline("mock-pipeline", tc.inProvider, tc.inStages)
			require.NoError(t, err)

			// WHEN
			b, err := manifest.MarshalBinary()

			// THEN
			require.Equal(t, string(wantedBytes), string(b))
			require.NoError(t, err)
		})
	}
}
