// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/stretchr/testify/require"
)

func TestScheduledJob_ApplyEnv_On(t *testing.T) {
	testCases := map[string]struct {
		inJob  func(job *ScheduledJob)
		wanted func(job *ScheduledJob)
	}{
		"schedule overridden": {
			inJob: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
				job.Environments["test"].On = JobTriggerConfig{
					Schedule: aws.String("mockScheduleTest"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockScheduleTest"),
				}
			},
		},
		"schedule explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
				job.Environments["test"].On = JobTriggerConfig{
					Schedule: aws.String(""),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String(""),
				}
			},
		},
		"schedule not overridden": {
			inJob: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Run(name, func(t *testing.T) {
				var inJob, wantedJob ScheduledJob
				inJob.Environments = map[string]*ScheduledJobConfig{
					"test": {},
				}

				tc.inJob(&inJob)
				tc.wanted(&wantedJob)

				got, err := inJob.ApplyEnv("test")

				require.NoError(t, err)
				require.Equal(t, &wantedJob, got)
			})
		})
	}
}

func TestScheduledJob_ApplyEnv_New(t *testing.T) {
	testCases := map[string]struct {
		inJob  func(job *ScheduledJob)
		wanted func(job *ScheduledJob)
	}{
		"on overridden": {
			inJob: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
				job.Environments["test"].On = JobTriggerConfig{
					Schedule: aws.String("mockScheduleTest"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockScheduleTest"),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: on not overridden": {
			inJob: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.On = JobTriggerConfig{
					Schedule: aws.String("mockSchedule"),
				}
			},
		},
		"image overridden": {
			inJob: func(job *ScheduledJob) {
				job.ImageConfig = ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
				job.Environments["test"].ImageConfig = ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("mockLocationTest"),
					},
				}
			},
			wanted: func(job *ScheduledJob) {
				job.ImageConfig = ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("mockLocationTest"),
					},
				}
			},
		},
		"image not overridden": {
			inJob: func(job *ScheduledJob) {
				job.ImageConfig = ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
			},
			wanted: func(job *ScheduledJob) {
				job.ImageConfig = ImageWithHealthcheck{
					Image: Image{
						Location: aws.String("mockLocation"),
					},
				}
			},
		},
		"entrypoint overridden": {
			inJob: func(job *ScheduledJob) {
				job.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				job.Environments["test"].EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: entrypoint not overridden": {
			inJob: func(job *ScheduledJob) {
				job.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
			},
		},
		"command overridden": {
			inJob: func(job *ScheduledJob) {
				job.Command = CommandOverride{
					String: aws.String("mock command"),
				}
				job.Environments["test"].Command = CommandOverride{
					String: aws.String("mock command test"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Command = CommandOverride{
					String: aws.String("mock command test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: command not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Command = CommandOverride{
					String: aws.String("mock command"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Command = CommandOverride{
					String: aws.String("mock command"),
				}
			},
		},
		"cpu overridden": {
			inJob: func(job *ScheduledJob) {
				job.CPU = aws.Int(1024)
				job.Environments["test"].CPU = aws.Int(2048)
			},
			wanted: func(job *ScheduledJob) {
				job.CPU = aws.Int(2048)
			},
		},
		"cpu explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.CPU = aws.Int(1024)
				job.Environments["test"].CPU = aws.Int(0)
			},
			wanted: func(job *ScheduledJob) {
				job.CPU = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: cpu not overridden": {
			inJob: func(job *ScheduledJob) {
				job.CPU = aws.Int(1024)
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.CPU = aws.Int(1024)
			},
		},
		"memory overridden": {
			inJob: func(job *ScheduledJob) {
				job.Memory = aws.Int(1024)
				job.Environments["test"].Memory = aws.Int(2048)
			},
			wanted: func(job *ScheduledJob) {
				job.Memory = aws.Int(2048)
			},
		},
		"memory explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.Memory = aws.Int(1024)
				job.Environments["test"].Memory = aws.Int(0)
			},
			wanted: func(job *ScheduledJob) {
				job.Memory = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: memory not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Memory = aws.Int(1024)
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Memory = aws.Int(1024)
			},
		},
		"platform overridden": {
			inJob: func(job *ScheduledJob) {
				job.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
				job.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform test"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
			},
		},
		"retries overridden": {
			inJob: func(job *ScheduledJob) {
				job.Retries = aws.Int(4)
				job.Environments["test"].Retries = aws.Int(42)
			},
			wanted: func(job *ScheduledJob) {
				job.Retries = aws.Int(42)
			},
		},
		"retries explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.Retries = aws.Int(4)
				job.Environments["test"].Retries = aws.Int(0)
			},
			wanted: func(job *ScheduledJob) {
				job.Retries = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: retries not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Retries = aws.Int(4)
				job.Environments["test"].JobFailureHandlerConfig = JobFailureHandlerConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Retries = aws.Int(4)
			},
		},
		"timeout overridden": {
			inJob: func(job *ScheduledJob) {
				job.Timeout = aws.String("mockTimeout")
				job.Environments["test"].Timeout = aws.String("mockTimeoutTest")
			},
			wanted: func(job *ScheduledJob) {
				job.Timeout = aws.String("mockTimeoutTest")
			},
		},
		"timeout explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.Timeout = aws.String("mockTimeout")
				job.Environments["test"].Timeout = aws.String("")
			},
			wanted: func(job *ScheduledJob) {
				job.Timeout = aws.String("")
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: timeout not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Timeout = aws.String("mockTimeout")
				job.Environments["test"].JobFailureHandlerConfig = JobFailureHandlerConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Timeout = aws.String("mockTimeout")
			},
		},
		"exec overridden": {
			inJob: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
				job.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"exec explicitly overridden by zero value": {
			inJob: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
				job.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: exec not overridden": {
			inJob: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true), // `false` is the zero value of pointer-to-bool
				}
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"network overridden": {
			inJob: func(job *ScheduledJob) {
				job.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				job.Environments["test"].Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: network not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Network = &NetworkConfig{
					VPC: &vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
		},
		"variables overridden": {
			inJob: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				job.Environments["test"].Variables = map[string]string{
					"mockVar1": "3", // Override the value of mockVar1
					"mockVar3": "3", // Append a new variable mockVar3
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "3",
					"mockVar2": "2",
					"mockVar3": "3",
				}
			},
		},
		"variables not overridden by empty map": {
			inJob: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				job.Environments["test"].Variables = map[string]string{}
			},
			wanted: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"variables not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"secrets overridden": {
			inJob: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				job.Environments["test"].Secrets = map[string]string{
					"mockSecret1": "3", // Override the value of mockSecret1
					"mockSecret3": "3", // Append a new variable mockSecret3
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "3",
					"mockSecret2": "2",
					"mockSecret3": "3",
				}
			},
		},
		"secrets not overridden by empty map": {
			inJob: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				job.Environments["test"].Secrets = map[string]string{}
			},
			wanted: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"secrets not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"storage overridden": {
			inJob: func(job *ScheduledJob) {
				job.Storage = &Storage{
					Ephemeral: aws.Int(3),
				}
				job.Environments["test"].Storage = &Storage{
					Ephemeral: aws.Int(5),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Storage = &Storage{
					Ephemeral: aws.Int(5),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: storage not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Storage = &Storage{
					Ephemeral: aws.Int(3),
				}
				job.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(job *ScheduledJob) {
				job.Storage = &Storage{
					Ephemeral: aws.Int(3),
				}
			},
		},
		"logging overridden": {
			inJob: func(job *ScheduledJob) {
				job.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
				job.Environments["test"].Logging = &Logging{
					Image: aws.String("mockImageTest"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Logging = &Logging{
					Image: aws.String("mockImageTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: logging not overridden": {
			inJob: func(job *ScheduledJob) {
				job.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
			},
			wanted: func(job *ScheduledJob) {
				job.Logging = &Logging{
					Image: aws.String("mockImage"),
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Run(name, func(t *testing.T) {
				var inJob, wantedJob ScheduledJob
				inJob.Environments = map[string]*ScheduledJobConfig{
					"test": {},
				}

				tc.inJob(&inJob)
				tc.wanted(&wantedJob)

				got, err := inJob.ApplyEnv("test")

				require.NoError(t, err)
				require.Equal(t, &wantedJob, got)
			})
		})
	}
}
