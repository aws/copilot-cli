// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0s

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/stretchr/testify/require"
)

func TestBackendSvc_ApplyEnv_New(t *testing.T) {
	testCases := map[string]struct {
		inSvc  func(svc *BackendService)
		wanted func(svc *BackendService)
	}{
		"image overridden": {
			inSvc: func(svc *BackendService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
				}
				svc.Environments["test"].ImageConfig = ImageWithPortAndHealthcheck{
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
			},
			wanted: func(svc *BackendService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
					HealthCheck: ContainerHealthCheck{
						Retries: aws.Int(3),
					},
				}
			},
		},
		"image not overridden": {
			inSvc: func(svc *BackendService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
				}
				svc.Environments["test"] = &BackendServiceConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.ImageConfig = ImageWithPortAndHealthcheck{
					ImageWithPort: ImageWithPort{
						Image: Image{
							Location: aws.String("mockLocation"),
						},
					},
				}
			},
		},
		"entrypoint overridden": {
			inSvc: func(svc *BackendService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
			wanted: func(svc *BackendService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: entrypoint not overridden": {
			inSvc: func(svc *BackendService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *BackendService) {
				svc.EntryPoint = EntryPointOverride{
					String: aws.String("mock entrypoint"),
				}
			},
		},
		"command overridden": {
			inSvc: func(svc *BackendService) {
				svc.Command = CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].Command = CommandOverride{
					String: aws.String("mock command test"),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Command = CommandOverride{
					String: aws.String("mock command test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: command not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Command = CommandOverride{
					String: aws.String("mock command"),
				}
				svc.Environments["test"].ImageOverride = ImageOverride{}
			},
			wanted: func(svc *BackendService) {
				svc.Command = CommandOverride{
					String: aws.String("mock command"),
				}
			},
		},
		"cpu overridden": {
			inSvc: func(svc *BackendService) {
				svc.CPU = aws.Int(1024)
				svc.Environments["test"].CPU = aws.Int(2048)
			},
			wanted: func(svc *BackendService) {
				svc.CPU = aws.Int(2048)
			},
		},
		"cpu explicitly overridden by zero value": {
			inSvc: func(svc *BackendService) {
				svc.CPU = aws.Int(1024)
				svc.Environments["test"].CPU = aws.Int(0)
			},
			wanted: func(svc *BackendService) {
				svc.CPU = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: cpu not overridden": {
			inSvc: func(svc *BackendService) {
				svc.CPU = aws.Int(1024)
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.CPU = aws.Int(1024)
			},
		},
		"memory overridden": {
			inSvc: func(svc *BackendService) {
				svc.Memory = aws.Int(1024)
				svc.Environments["test"].Memory = aws.Int(2048)
			},
			wanted: func(svc *BackendService) {
				svc.Memory = aws.Int(2048)
			},
		},
		"memory explicitly overridden by zero value": {
			inSvc: func(svc *BackendService) {
				svc.Memory = aws.Int(1024)
				svc.Environments["test"].Memory = aws.Int(0)
			},
			wanted: func(svc *BackendService) {
				svc.Memory = aws.Int(0)
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: memory not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Memory = aws.Int(1024)
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Memory = aws.Int(1024)
			},
		},
		"platform overridden": {
			inSvc: func(svc *BackendService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
				svc.Environments["test"].Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform test"),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform test"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: platform not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Platform = &PlatformArgsOrString{
					PlatformString: aws.String("mock platform"),
				}
			},
		},
		"count overridden": {
			inSvc: func(svc *BackendService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
				svc.Environments["test"].Count = Count{
					Value: aws.Int(5),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Count = Count{
					Value: aws.Int(5),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: count not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Count = Count{
					Value: aws.Int(3),
				}
			},
		},
		"exec overridden": {
			inSvc: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
				svc.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
			wanted: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"exec explicitly overridden by zero value": {
			inSvc: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
				svc.Environments["test"].ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
			wanted: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(false),
				}
			},
		},
		"FIXED_AFTER_TRANSFORM_POINTER: exec not overridden": {
			inSvc: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.ExecuteCommand = ExecuteCommand{
					Enable: aws.Bool(true),
				}
			},
		},
		"network overridden": {
			inSvc: func(svc *BackendService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
				svc.Environments["test"].Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
			wanted: func(svc *BackendService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacementTest"),
					},
				}
			},
		},
		"FAILED_AFTER_UPGRADE: network not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
			wanted: func(svc *BackendService) {
				svc.Network = NetworkConfig{
					VPC: vpcConfig{
						Placement: aws.String("mockPlacement"),
					},
				}
			},
		},
		"variables overridden": {
			inSvc: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{
					"mockVar1": "3", // Override the value of mockVar1
					"mockVar3": "3", // Append a new variable mockVar3
				}
			},
			wanted: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "3",
					"mockVar2": "2",
					"mockVar3": "3",
				}
			},
		},
		"variables not overridden by empty map": {
			inSvc: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].Variables = map[string]string{}
			},
			wanted: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"variables not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Variables = map[string]string{
					"mockVar1": "1",
					"mockVar2": "2",
				}
			},
		},
		"secrets overridden": {
			inSvc: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].Secrets = map[string]string{
					"mockSecret1": "3", // Override the value of mockSecret1
					"mockSecret3": "3", // Append a new variable mockSecret3
				}
			},
			wanted: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "3",
					"mockSecret2": "2",
					"mockSecret3": "3",
				}
			},
		},
		"secrets not overridden by empty map": {
			inSvc: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].Secrets = map[string]string{}
			},
			wanted: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"secrets not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Secrets = map[string]string{
					"mockSecret1": "1",
					"mockSecret2": "2",
				}
			},
		},
		"storage overridden": {
			inSvc: func(svc *BackendService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
				svc.Environments["test"].Storage = Storage{
					Ephemeral: aws.Int(5),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(5),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: storage not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
				svc.Environments["test"].TaskConfig = TaskConfig{}
			},
			wanted: func(svc *BackendService) {
				svc.Storage = Storage{
					Ephemeral: aws.Int(3),
				}
			},
		},
		"logging overridden": {
			inSvc: func(svc *BackendService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
				svc.Environments["test"].Logging = Logging{
					Image: aws.String("mockImageTest"),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Logging = Logging{
					Image: aws.String("mockImageTest"),
				}
			},
		},
		"FAILED_AFTER_UPGRADE: logging not overridden": {
			inSvc: func(svc *BackendService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
			},
			wanted: func(svc *BackendService) {
				svc.Logging = Logging{
					Image: aws.String("mockImage"),
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var inSvc, wantedSvc BackendService
			inSvc.Environments = map[string]*BackendServiceConfig{
				"test": {},
			}

			tc.inSvc(&inSvc)
			tc.wanted(&wantedSvc)

			got, err := inSvc.ApplyEnv("test")

			require.NoError(t, err)
			require.Equal(t, &wantedSvc, got)
		})
	}
}
