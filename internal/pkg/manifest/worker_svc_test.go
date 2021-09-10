// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNewWorkerSvc(t *testing.T) {
	testCases := map[string]struct {
		inProps WorkerServiceProps

		wantedManifest *WorkerService
	}{
		"should return a worker service instance": {
			inProps: WorkerServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "testers",
					Dockerfile: "./testers/Dockerfile",
				},
			},
			wantedManifest: &WorkerService{
				Workload: Workload{
					Name: aws.String("testers"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./testers/Dockerfile"),
								},
							},
						},
					},
					Subscribe: SubscribeConfig{},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
						},
						ExecuteCommand: ExecuteCommand{
							Enable: aws.Bool(false),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: stringP("public"),
						},
					},
				},
			},
		},
		"should return a worker service instance with subscribe": {
			inProps: WorkerServiceProps{
				WorkloadProps: WorkloadProps{
					Name:       "testers",
					Dockerfile: "./testers/Dockerfile",
				},
			},
			wantedManifest: &WorkerService{
				Workload: Workload{
					Name: aws.String("testers"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildArgs: DockerBuildArgs{
									Dockerfile: aws.String("./testers/Dockerfile"),
								},
							},
						},
					},
					Subscribe: SubscribeConfig{},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(256),
						Memory: aws.Int(512),
						Count: Count{
							Value: aws.Int(1),
						},
						ExecuteCommand: ExecuteCommand{
							Enable: aws.Bool(false),
						},
					},
					Network: NetworkConfig{
						VPC: vpcConfig{
							Placement: stringP("public"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			wantedBytes, err := yaml.Marshal(tc.wantedManifest)
			require.NoError(t, err)

			// WHEN
			actualBytes, err := yaml.Marshal(NewWorkerService(tc.inProps))
			require.NoError(t, err)

			require.Equal(t, string(wantedBytes), string(actualBytes))
		})
	}
}

func TestWorkerSvc_MarshalBinary(t *testing.T) {
	testCases := map[string]struct {
		inProps WorkerServiceProps

		wantedTestdata string
	}{
		"without subscribe": {
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
			},
			wantedTestdata: "worker-svc-nosubscribe.yml",
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
						Name:    "testTopic",
						Service: "service4TestTopic",
					},
					{
						Name:    "testTopic2",
						Service: "service4TestTopic2",
					},
				},
			},
			wantedTestdata: "worker-svc-subscribe.yml",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			path := filepath.Join("testdata", tc.wantedTestdata)
			wantedBytes, err := ioutil.ReadFile(path)
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

func TestWorkerSvc_ApplyEnv(t *testing.T) {
	mockWorkerServiceWithNoEnvironments := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{
					Build: BuildArgsOrString{
						BuildArgs: DockerBuildArgs{
							Dockerfile: aws.String("./Dockerfile"),
						},
					},
				},
				HealthCheck: ContainerHealthCheck{
					Command:     []string{"hello", "world"},
					Interval:    durationp(1 * time.Second),
					Retries:     aws.Int(100),
					Timeout:     durationp(100 * time.Minute),
					StartPeriod: durationp(5 * time.Second),
				},
			},
			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(256),
				Count: Count{
					Value: aws.Int(1),
				},
			},
		},
	}
	mockWorkerServiceWithNilEnvironment := WorkerService{
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test": nil,
		},
	}
	mockWorkerServiceWithMinimalOverride := WorkerService{
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{},
				},
			},
		},
	}
	mockWorkerServiceWithAllOverride := WorkerService{
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{
					DockerLabels: map[string]string{
						"com.amazonaws.ecs.copilot.description": "Hello world!",
					},
				},
			},

			TaskConfig: TaskConfig{
				CPU:    aws.Int(256),
				Memory: aws.Int(256),
				Count: Count{
					Value: aws.Int(1),
				},
			},
			Sidecars: map[string]*SidecarConfig{
				"xray": {
					Port:  aws.String("2000/udp"),
					Image: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
				},
			},
			Logging: Logging{
				Destination: map[string]string{
					"Name":            "datadog",
					"exclude-pattern": "*",
				},
			},
			Subscribe: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name:    "topicName",
						Service: "bestService",
					},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						DockerLabels: map[string]string{
							"com.amazonaws.ecs.copilot.description": "Overridden!",
						},
					},
				},
				TaskConfig: TaskConfig{
					Count: Count{
						AdvancedCount: AdvancedCount{
							CPU: aws.Int(70),
						},
					},
					CPU: aws.Int(512),
					Variables: map[string]string{
						"LOG_LEVEL": "",
					},
				},
				Sidecars: map[string]*SidecarConfig{
					"xray": {
						CredsParam: aws.String("some arn"),
					},
				},
				Logging: Logging{
					Destination: map[string]string{
						"include-pattern": "*",
						"exclude-pattern": "fe/",
					},
				},
				Subscribe: SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "topicName2",
							Service: "bestService2",
						},
					},
				},
			},
		},
	}
	mockWorkerServiceWithImageOverrideBuildByLocation := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
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
		Environments: map[string]*WorkerServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("env-override location"),
					},
				},
			},
		},
	}
	mockWorkerServiceWithImageOverrideLocationByLocation := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{
					Location: aws.String("original location"),
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("env-override location"),
					},
				},
			},
		},
	}
	mockWorkerServiceWithImageOverrideBuildByBuild := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{
					Build: BuildArgsOrString{
						BuildArgs: DockerBuildArgs{
							Dockerfile: aws.String("original dockerfile"),
							Context:    aws.String("original context"),
						},
					},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Build: BuildArgsOrString{
							BuildString: aws.String("env overridden dockerfile"),
						},
					},
				},
			},
		},
	}
	mockWorkerServiceWithImageOverrideLocationByBuild := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			ImageConfig: ImageWithHealthcheck{
				Image: Image{
					Location: aws.String("original location"),
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"prod-iad": {
				ImageConfig: ImageWithHealthcheck{
					Image: Image{
						Build: BuildArgsOrString{
							BuildString: aws.String("env overridden dockerfile"),
						},
					},
				},
			},
		},
	}
	duration111Seconds := 111 * time.Second
	mockWorkerServiceWithSubscribeNilOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name:    "name",
						Service: "svc",
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{},
			},
		},
	}
	mockWorkerServiceWithNilSubscribeOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "name",
							Service: "svc",
							Queue: &SQSQueue{
								Retention:  &duration111Seconds,
								Delay:      &duration111Seconds,
								Timeout:    &duration111Seconds,
								DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
							},
						},
					},
				},
			},
		},
	}
	mockWorkerServiceWithEmptySubscribeOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name:    "name",
						Service: "svc",
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{},
			},
		},
	}
	mockWorkerServiceWithSubscribeTopicNilOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Topics: nil,
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "name",
							Service: "svc",
							Queue: &SQSQueue{
								Retention:  &duration111Seconds,
								Delay:      &duration111Seconds,
								Timeout:    &duration111Seconds,
								DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
							},
						},
					},
				},
			},
		},
	}
	mockWorkerServiceWithNilSubscribeTopicOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name:    "name",
						Service: "svc",
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Topics: nil,
				},
			},
		},
	}
	mockWorkerServiceWithSubscribeTopicEmptyOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Topics: []TopicSubscription{},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Topics: []TopicSubscription{
						{
							Name:    "name",
							Service: "svc",
							Queue: &SQSQueue{
								Retention:  &duration111Seconds,
								Delay:      &duration111Seconds,
								Timeout:    &duration111Seconds,
								DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
							},
						},
					},
				},
			},
		},
	}
	mockWorkerServiceWithSubscribeQueueNilOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Queue: nil,
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Queue: &SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
					},
				},
			},
		},
	}
	mockWorkerServiceWithNilSubscribeQueueOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Queue: &SQSQueue{
					Retention:  &duration111Seconds,
					Delay:      &duration111Seconds,
					Timeout:    &duration111Seconds,
					DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
				},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Queue: nil,
				},
			},
		},
	}
	mockWorkerServiceWithSubscribeQueueEmptyOverride := WorkerService{
		Workload: Workload{
			Name: aws.String("phonetool"),
			Type: aws.String(WorkerServiceType),
		},
		WorkerServiceConfig: WorkerServiceConfig{
			Subscribe: SubscribeConfig{
				Queue: &SQSQueue{},
			},
		},
		Environments: map[string]*WorkerServiceConfig{
			"test-sub": {
				Subscribe: SubscribeConfig{
					Queue: &SQSQueue{
						Retention:  &duration111Seconds,
						Delay:      &duration111Seconds,
						Timeout:    &duration111Seconds,
						DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
					},
				},
			},
		},
	}
	testCases := map[string]struct {
		svc       *WorkerService
		inEnvName string

		wanted   *WorkerService
		original *WorkerService
	}{
		"no env override": {
			svc:       &mockWorkerServiceWithNoEnvironments,
			inEnvName: "test",

			wanted:   &mockWorkerServiceWithNoEnvironments,
			original: &mockWorkerServiceWithNoEnvironments,
		},
		"with nil env override": {
			svc:       &mockWorkerServiceWithNilEnvironment,
			inEnvName: "test",

			wanted:   &mockWorkerServiceWithNilEnvironment,
			original: &mockWorkerServiceWithNilEnvironment,
		},
		"uses env minimal overrides": {
			svc:       &mockWorkerServiceWithMinimalOverride,
			inEnvName: "test",

			wanted: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{},
					},
				},
			},
			original: &mockWorkerServiceWithMinimalOverride,
		},
		"uses env all overrides": {
			svc:       &mockWorkerServiceWithAllOverride,
			inEnvName: "test",

			wanted: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							DockerLabels: map[string]string{
								"com.amazonaws.ecs.copilot.description": "Overridden!",
							},
						},
					},
					TaskConfig: TaskConfig{
						CPU:    aws.Int(512),
						Memory: aws.Int(256),
						Count: Count{
							AdvancedCount: AdvancedCount{
								CPU: aws.Int(70),
							},
						},
						Variables: map[string]string{
							"LOG_LEVEL": "",
						},
					},
					Sidecars: map[string]*SidecarConfig{
						"xray": {
							Port:       aws.String("2000/udp"),
							Image:      aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
							CredsParam: aws.String("some arn"),
						},
					},
					Logging: Logging{
						Destination: map[string]string{
							"Name":            "datadog",
							"include-pattern": "*",
							"exclude-pattern": "fe/",
						},
					},
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "topicName2",
								Service: "bestService2",
							},
						},
					},
				},
			},
			original: &mockWorkerServiceWithAllOverride,
		},
		"with image build overridden by image location": {
			svc:       &mockWorkerServiceWithImageOverrideBuildByLocation,
			inEnvName: "prod-iad",

			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
			original: &mockWorkerServiceWithImageOverrideBuildByLocation,
		},
		"with image location overridden by image location": {
			svc:       &mockWorkerServiceWithImageOverrideLocationByLocation,
			inEnvName: "prod-iad",

			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Location: aws.String("env-override location"),
						},
					},
				},
			},
			original: &mockWorkerServiceWithImageOverrideLocationByLocation,
		},
		"with image build overridden by image build": {
			svc:       &mockWorkerServiceWithImageOverrideBuildByBuild,
			inEnvName: "prod-iad",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("env overridden dockerfile"),
							},
						},
					},
				},
			},
			original: &mockWorkerServiceWithImageOverrideBuildByBuild,
		},
		"with image location overridden by image build": {
			svc:       &mockWorkerServiceWithImageOverrideLocationByBuild,
			inEnvName: "prod-iad",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{
							Build: BuildArgsOrString{
								BuildString: aws.String("env overridden dockerfile"),
							},
						},
					},
				},
			},
			original: &mockWorkerServiceWithImageOverrideLocationByBuild,
		},
		"with nil subscribe overriden by full subscribe": {
			svc:       &mockWorkerServiceWithNilSubscribeOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithNilSubscribeOverride,
		},
		"with full subscribe and nil subscribe env": {
			svc:       &mockWorkerServiceWithSubscribeNilOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithSubscribeNilOverride,
		},
		"with full subscribe and empty subscribe env": {
			svc:       &mockWorkerServiceWithEmptySubscribeOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{},
					},
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithEmptySubscribeOverride,
		},
		"with nil subscribe topic overriden by full subscribe topic": {
			svc:       &mockWorkerServiceWithNilSubscribeTopicOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithNilSubscribeTopicOverride,
		},
		"with full subscribe topic and nil subscribe topic env": {
			svc:       &mockWorkerServiceWithSubscribeTopicNilOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithSubscribeTopicNilOverride,
		},
		"with empty subscribe topic overriden by full subscribe topic": {
			svc:       &mockWorkerServiceWithSubscribeTopicEmptyOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{},
					},
					Subscribe: SubscribeConfig{
						Topics: []TopicSubscription{
							{
								Name:    "name",
								Service: "svc",
								Queue: &SQSQueue{
									Retention:  &duration111Seconds,
									Delay:      &duration111Seconds,
									Timeout:    &duration111Seconds,
									DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
								},
							},
						},
					},
				},
			},

			original: &mockWorkerServiceWithSubscribeTopicEmptyOverride,
		},
		"with nil subscribe queue overriden by full subscribe queue": {
			svc:       &mockWorkerServiceWithNilSubscribeQueueOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},

			original: &mockWorkerServiceWithNilSubscribeQueueOverride,
		},
		"with full subscribe queue and nil subscribe queue env": {
			svc:       &mockWorkerServiceWithSubscribeQueueNilOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					Subscribe: SubscribeConfig{
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},

			original: &mockWorkerServiceWithSubscribeQueueNilOverride,
		},
		"with empty subscribe queue overridden by full subscribe queue": {
			svc:       &mockWorkerServiceWithSubscribeQueueEmptyOverride,
			inEnvName: "test-sub",
			wanted: &WorkerService{
				Workload: Workload{
					Name: aws.String("phonetool"),
					Type: aws.String(WorkerServiceType),
				},
				WorkerServiceConfig: WorkerServiceConfig{
					ImageConfig: ImageWithHealthcheck{
						Image: Image{},
					},
					Subscribe: SubscribeConfig{
						Queue: &SQSQueue{
							Retention:  &duration111Seconds,
							Delay:      &duration111Seconds,
							Timeout:    &duration111Seconds,
							DeadLetter: DeadLetterQueue{Tries: aws.Uint16(10)},
						},
					},
				},
			},

			original: &mockWorkerServiceWithSubscribeQueueEmptyOverride,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, _ := tc.svc.ApplyEnv(tc.inEnvName)

			// Should override properly.
			require.Equal(t, tc.wanted, got)
			// Should not impact the original manifest struct.
			require.Equal(t, tc.svc, tc.original)
		})
	}
}

func TestWorkerSvc_ApplyEnv_CountOverrides(t *testing.T) {
	mockRange := IntRangeBand("1-10")
	testCases := map[string]struct {
		svcCount Count
		envCount Count

		expected *WorkerService
	}{
		"empty env advanced count override": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{Value: &mockRange},
					CPU:   aws.Int(80),
				},
			},
			envCount: Count{},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: &Range{Value: &mockRange},
								CPU:   aws.Int(80),
							},
						},
					},
				},
			},
		},
		"with count value overriden by count value": {
			svcCount: Count{Value: aws.Int(5)},
			envCount: Count{Value: aws.Int(8)},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(8)},
					},
				},
			},
		},
		"with count value overriden by spot count": {
			svcCount: Count{Value: aws.Int(4)},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(6),
				},
			},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot: aws.Int(6),
							},
						},
					},
				},
			},
		},
		"with range overriden by spot count": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{Value: &mockRange},
				},
			},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(6),
				},
			},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Spot: aws.Int(6),
							},
						},
					},
				},
			},
		},
		"with range overriden by range config": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{Value: &mockRange},
				},
			},
			envCount: Count{
				AdvancedCount: AdvancedCount{
					Range: &Range{
						RangeConfig: RangeConfig{
							Min: aws.Int(2),
							Max: aws.Int(8),
						},
					},
				},
			},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{
							AdvancedCount: AdvancedCount{
								Range: &Range{
									RangeConfig: RangeConfig{
										Min: aws.Int(2),
										Max: aws.Int(8),
									},
								},
							},
						},
					},
				},
			},
		},
		"with spot overriden by count value": {
			svcCount: Count{
				AdvancedCount: AdvancedCount{
					Spot: aws.Int(5),
				},
			},
			envCount: Count{Value: aws.Int(12)},
			expected: &WorkerService{
				WorkerServiceConfig: WorkerServiceConfig{
					TaskConfig: TaskConfig{
						Count: Count{Value: aws.Int(12)},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		// GIVEN
		svc := WorkerService{
			WorkerServiceConfig: WorkerServiceConfig{
				TaskConfig: TaskConfig{
					Count: tc.svcCount,
				},
			},
			Environments: map[string]*WorkerServiceConfig{
				"test": {
					TaskConfig: TaskConfig{
						Count: tc.envCount,
					},
				},
				"staging": {
					TaskConfig: TaskConfig{},
				},
			},
		}
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual, _ := svc.ApplyEnv("test")

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestWorkerSvc_Subscriptions(t *testing.T) {
	testCases := map[string]struct {
		inSubscription SubscribeConfig
		expected       []TopicSubscription
	}{
		"subscription specified": {
			inSubscription: SubscribeConfig{
				Topics: []TopicSubscription{
					{
						Name:    "orders",
						Service: "database",
					},
					{
						Name:    "events",
						Service: "api",
					},
				},
				Queue: &SQSQueue{
					Retention: durationPointer(4 * 24 * time.Hour),
				},
			},
			expected: []TopicSubscription{
				{
					Name:    "orders",
					Service: "database",
				},
				{
					Name:    "events",
					Service: "api",
				},
			},
		},
		"no topics": {
			inSubscription: SubscribeConfig{},
			expected:       nil,
		},
	}
	for name, tc := range testCases {
		// GIVEN
		svc := WorkerService{
			WorkerServiceConfig: WorkerServiceConfig{
				Subscribe: tc.inSubscription,
			},
		}
		t.Run(name, func(t *testing.T) {
			// WHEN
			actual := svc.Subscriptions()

			// THEN
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestDeadLetterQueue_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     DeadLetterQueue
		wanted bool
	}{
		"empty dead letter queue": {
			in:     DeadLetterQueue{},
			wanted: true,
		},
		"non empty dead letter queue": {
			in: DeadLetterQueue{
				Tries: aws.Uint16(3),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.IsEmpty()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}
