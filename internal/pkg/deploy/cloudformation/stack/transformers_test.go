// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/template/override"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_convertSidecar(t *testing.T) {
	mockImage := aws.String("mockImage")
	mockMap := map[string]string{"foo": "bar"}
	mockSecrets := map[string]template.Secret{"foo": template.SecretFromSSMOrARN("")}
	mockCredsParam := aws.String("mockCredsParam")
	testCases := map[string]struct {
		inPort            *string
		inEssential       bool
		inLabels          map[string]string
		inDependsOn       map[string]string
		inImageOverride   manifest.ImageOverride
		inHealthCheck     manifest.ContainerHealthCheck
		circDepContainers []string

		wanted    *template.SidecarOpts
		wantedErr error
	}{
		"invalid port": {
			inPort: aws.String("b/a/d/P/o/r/t"),

			wantedErr: fmt.Errorf("cannot parse port mapping from b/a/d/P/o/r/t"),
		},
		"good port without protocol": {
			inPort:      aws.String("2000"),
			inEssential: true,

			wanted: &template.SidecarOpts{
				Name:       "foo",
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"good port with protocol": {
			inPort:      aws.String("2000/udp"),
			inEssential: true,

			wanted: &template.SidecarOpts{
				Name:       "foo",
				Port:       aws.String("2000"),
				Protocol:   aws.String("udp"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"good container dependencies": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"frontend": "start",
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
				DependsOn: map[string]string{
					"frontend": "START",
				},
			},
		},
		"specify essential as false": {
			inPort:      aws.String("2000"),
			inEssential: false,
			inLabels: map[string]string{
				"com.amazonaws.ecs.copilot.sidecar.description": "wow",
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				DockerLabels: map[string]string{
					"com.amazonaws.ecs.copilot.sidecar.description": "wow",
				},
			},
		},
		"do not specify image override": {
			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    nil,
			},
		},
		"specify entrypoint as a string": {
			inImageOverride: manifest.ImageOverride{
				EntryPoint: manifest.EntryPointOverride{String: aws.String("bin")},
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: []string{"bin"},
				Command:    nil,
			},
		},
		"specify entrypoint as a string slice": {
			inImageOverride: manifest.ImageOverride{
				EntryPoint: manifest.EntryPointOverride{StringSlice: []string{"bin", "arg"}},
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: []string{"bin", "arg"},
				Command:    nil,
			},
		},
		"specify command as a string": {
			inImageOverride: manifest.ImageOverride{
				Command: manifest.CommandOverride{String: aws.String("arg")},
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    []string{"arg"},
			},
		},
		"specify command as a string slice": {
			inImageOverride: manifest.ImageOverride{
				Command: manifest.CommandOverride{StringSlice: []string{"arg1", "arg2"}},
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    []string{"arg1", "arg2"},
			},
		},
		"with health check": {
			inHealthCheck: manifest.ContainerHealthCheck{
				Command: []string{"foo", "bar"},
			},

			wanted: &template.SidecarOpts{
				Name:       "foo",
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockSecrets,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				HealthCheck: &template.ContainerHealthCheck{
					Command:     []string{"foo", "bar"},
					Interval:    aws.Int64(10),
					Retries:     aws.Int64(2),
					StartPeriod: aws.Int64(0),
					Timeout:     aws.Int64(5),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sidecar := map[string]*manifest.SidecarConfig{
				"foo": {
					CredsParam:    mockCredsParam,
					Image:         mockImage,
					Secrets:       map[string]manifest.Secret{"foo": {}},
					Variables:     mockMap,
					Essential:     aws.Bool(tc.inEssential),
					Port:          tc.inPort,
					DockerLabels:  tc.inLabels,
					DependsOn:     tc.inDependsOn,
					ImageOverride: tc.inImageOverride,
					HealthCheck:   tc.inHealthCheck,
				},
			}
			got, err := convertSidecar(sidecar, nil, nil, nil, nil)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got[0])
			}
		})
	}
}

func Test_convertAdvancedCount(t *testing.T) {
	mockRange := manifest.IntRangeBand("1-10")
	timeMinute := time.Second * 60
	perc := manifest.Percentage(70)
	mockConfig := manifest.ScalingConfigOrT[manifest.Percentage]{
		ScalingConfig: manifest.AdvancedScalingConfig[manifest.Percentage]{
			Value: &perc,
			Cooldown: manifest.Cooldown{
				ScaleInCooldown: &timeMinute,
			},
		},
	}
	testCases := map[string]struct {
		input       manifest.AdvancedCount
		expected    *template.AdvancedCount
		expectedErr error
	}{
		"success with generalized cooldown": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					Value: &mockRange,
				},
				Cooldown: manifest.Cooldown{
					ScaleInCooldown:  &timeMinute,
					ScaleOutCooldown: &timeMinute,
				},
			},
			expected: &template.AdvancedCount{
				Autoscaling: &template.AutoscalingOpts{
					MinCapacity: aws.Int(1),
					MaxCapacity: aws.Int(10),
					CPUCooldown: template.Cooldown{
						ScaleInCooldown:  aws.Float64(60),
						ScaleOutCooldown: aws.Float64(60),
					},
					MemCooldown: template.Cooldown{
						ScaleInCooldown:  aws.Float64(60),
						ScaleOutCooldown: aws.Float64(60),
					},
					ReqCooldown: template.Cooldown{
						ScaleInCooldown:  aws.Float64(60),
						ScaleOutCooldown: aws.Float64(60),
					},
					RespTimeCooldown: template.Cooldown{
						ScaleInCooldown:  aws.Float64(60),
						ScaleOutCooldown: aws.Float64(60),
					},
					QueueDelayCooldown: template.Cooldown{
						ScaleInCooldown:  aws.Float64(60),
						ScaleOutCooldown: aws.Float64(60),
					},
				},
			},
		},
		"success with spot count": {
			input: manifest.AdvancedCount{
				Spot: aws.Int(1),
			},
			expected: &template.AdvancedCount{
				Spot: aws.Int(1),
				Cps: []*template.CapacityProviderStrategy{
					{
						Weight:           aws.Int(1),
						CapacityProvider: capacityProviderFargateSpot,
					},
				},
			},
		},
		"success with fargate autoscaling": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					Value: &mockRange,
				},
				CPU: mockConfig,
			},
			expected: &template.AdvancedCount{
				Autoscaling: &template.AutoscalingOpts{
					MinCapacity: aws.Int(1),
					MaxCapacity: aws.Int(10),
					CPU:         aws.Float64(70),
					CPUCooldown: template.Cooldown{
						ScaleInCooldown: aws.Float64(60),
					},
				},
			},
		},
		"success with spot autoscaling": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(2),
						Max:      aws.Int(20),
						SpotFrom: aws.Int(5),
					},
				},
				CPU: mockConfig,
			},
			expected: &template.AdvancedCount{
				Autoscaling: &template.AutoscalingOpts{
					MinCapacity: aws.Int(2),
					MaxCapacity: aws.Int(20),
					CPU:         aws.Float64(70),
					CPUCooldown: template.Cooldown{
						ScaleInCooldown: aws.Float64(60),
					},
				},
				Cps: []*template.CapacityProviderStrategy{
					{
						Weight:           aws.Int(1),
						CapacityProvider: capacityProviderFargateSpot,
					},
					{
						Base:             aws.Int(4),
						Weight:           aws.Int(0),
						CapacityProvider: capacityProviderFargate,
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := convertAdvancedCount(tc.input)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func Test_convertCapacityProviders(t *testing.T) {
	mockRange := manifest.IntRangeBand("1-10")
	minCapacity := 1
	spotFrom := 3
	testCases := map[string]struct {
		input    manifest.AdvancedCount
		expected []*template.CapacityProviderStrategy
	}{
		"with spot as desiredCount": {
			input: manifest.AdvancedCount{
				Spot: aws.Int(3),
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
			},
		},
		"with scaling only on spot": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(minCapacity),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(minCapacity),
					},
				},
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
				{
					Base:             aws.Int(0),
					Weight:           aws.Int(0),
					CapacityProvider: capacityProviderFargate,
				},
			},
		},
		"with scaling into spot": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(minCapacity),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(spotFrom),
					},
				},
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
				{
					Base:             aws.Int(spotFrom - 1),
					Weight:           aws.Int(0),
					CapacityProvider: capacityProviderFargate,
				},
			},
		},
		"with min equaling spot_from": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(2),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(2),
					},
				},
			},

			expected: []*template.CapacityProviderStrategy{
				{
					Weight:           aws.Int(1),
					CapacityProvider: capacityProviderFargateSpot,
				},
				{
					Base:             aws.Int(1),
					Weight:           aws.Int(0),
					CapacityProvider: capacityProviderFargate,
				},
			},
		},
		"returns nil if no spot config specified": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					Value: &mockRange,
				},
			},
			expected: nil,
		},
		"returns nil if no spot config specified with min max": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min: aws.Int(1),
						Max: aws.Int(10),
					},
				},
			},
			expected: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := convertCapacityProviders(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func Test_convertAutoscaling(t *testing.T) {
	var (
		respTime     = 512 * time.Millisecond
		mockRange    = manifest.IntRangeBand("1-100")
		badRange     = manifest.IntRangeBand("badRange")
		mockRequests = manifest.ScalingConfigOrT[int]{
			Value: aws.Int(1000),
		}
		mockResponseTime = manifest.ScalingConfigOrT[time.Duration]{
			Value: &respTime,
		}
		perc       = manifest.Percentage(70)
		timeMinute = time.Second * 60
		mockCPU    = manifest.ScalingConfigOrT[manifest.Percentage]{
			ScalingConfig: manifest.AdvancedScalingConfig[manifest.Percentage]{
				Value: &perc,
				Cooldown: manifest.Cooldown{
					ScaleInCooldown:  &timeMinute,
					ScaleOutCooldown: &timeMinute,
				},
			},
		}
		mockMem = manifest.ScalingConfigOrT[manifest.Percentage]{
			Value: &perc,
		}
	)

	testAcceptableLatency := 10 * time.Minute
	testAvgProcessingTime := 250 * time.Millisecond
	testCases := map[string]struct {
		input manifest.AdvancedCount

		wanted    *template.AutoscalingOpts
		wantedErr error
	}{
		"invalid range": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					Value: &badRange,
				},
			},

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"success": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					Value: &mockRange,
				},
				CPU:          mockCPU,
				Memory:       mockMem,
				Requests:     mockRequests,
				ResponseTime: mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity: aws.Int(100),
				MinCapacity: aws.Int(1),
				CPU:         aws.Float64(70),
				Memory:      aws.Float64(70),
				CPUCooldown: template.Cooldown{
					ScaleInCooldown:  aws.Float64(60),
					ScaleOutCooldown: aws.Float64(60),
				},
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"success with range subfields": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(5),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(5),
					},
				},
				CPU:          mockCPU,
				Memory:       mockMem,
				Requests:     mockRequests,
				ResponseTime: mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity: aws.Int(10),
				MinCapacity: aws.Int(5),
				CPU:         aws.Float64(70),
				Memory:      aws.Float64(70),
				CPUCooldown: template.Cooldown{
					ScaleInCooldown:  aws.Float64(60),
					ScaleOutCooldown: aws.Float64(60),
				},
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"success with queue autoscaling": {
			input: manifest.AdvancedCount{
				Range: manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(5),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(5),
					},
				},
				QueueScaling: manifest.QueueScaling{
					AcceptableLatency: &testAcceptableLatency,
					AvgProcessingTime: &testAvgProcessingTime,
					Cooldown: manifest.Cooldown{
						ScaleInCooldown: &timeMinute,
					},
				},
			},
			wanted: &template.AutoscalingOpts{
				MaxCapacity: aws.Int(10),
				MinCapacity: aws.Int(5),
				QueueDelayCooldown: template.Cooldown{
					ScaleInCooldown: aws.Float64(60),
				},
				QueueDelay: &template.AutoscalingQueueDelayOpts{
					AcceptableBacklogPerTask: 2400,
				},
			},
		},
		"returns nil if spot specified": {
			input: manifest.AdvancedCount{
				Spot: aws.Int(5),
			},
			wanted: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := convertAutoscaling(tc.input)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, got)
			}
		})
	}
}

func Test_convertTaskDefOverrideRules(t *testing.T) {
	testCases := map[string]struct {
		inRule []manifest.OverrideRule

		wanted []override.Rule
	}{
		"should have proper prefix": {
			inRule: []manifest.OverrideRule{
				{
					Path:  "ContainerDefinitions[0].Ulimits[-].HardLimit",
					Value: yaml.Node{},
				},
			},
			wanted: []override.Rule{
				{
					Path:  "Resources.TaskDefinition.Properties.ContainerDefinitions[0].Ulimits[-].HardLimit",
					Value: yaml.Node{},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := convertTaskDefOverrideRules(tc.inRule)

			require.Equal(t, tc.wanted, got)
		})
	}
}

func Test_convertHTTPHealthCheck(t *testing.T) {
	// These are used by reference to represent the output of the manifest.durationp function.
	duration15Seconds := 15 * time.Second
	duration60Seconds := 60 * time.Second
	testCases := map[string]struct {
		input      manifest.HealthCheckArgsOrString
		wantedOpts template.HTTPHealthCheckOpts
	}{
		"no fields indicated in manifest": {
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				GracePeriod:     60,
			},
		},
		"just Path": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.BasicToUnion[string, manifest.HTTPHealthCheckArgs]("path"),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "path",
				GracePeriod:     60,
			},
		},
		"just HealthyThreshold": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					HealthyThreshold: aws.Int64(5),
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:  "/",
				HealthyThreshold: aws.Int64(5),
				GracePeriod:      60,
			},
		},
		"just UnhealthyThreshold": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					UnhealthyThreshold: aws.Int64(5),
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/",
				UnhealthyThreshold: aws.Int64(5),
				GracePeriod:        60,
			},
		},
		"just Interval": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					Interval: &duration15Seconds,
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Interval:        aws.Int64(15),
				GracePeriod:     60,
			},
		},
		"just Timeout": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					Timeout: &duration15Seconds,
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Timeout:         aws.Int64(15),
				GracePeriod:     60,
			},
		},
		"just SuccessCodes": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					SuccessCodes: aws.String("200,301"),
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				SuccessCodes:    "200,301",
				GracePeriod:     60,
			},
		},
		"just Port": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					Port: aws.Int(8000),
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Port:            "8000",
				GracePeriod:     60,
			},
		},
		"all values changed in manifest": {
			input: manifest.HealthCheckArgsOrString{
				Union: manifest.AdvancedToUnion[string](manifest.HTTPHealthCheckArgs{
					Path:               aws.String("/road/to/nowhere"),
					Port:               aws.Int(8080),
					SuccessCodes:       aws.String("200-299"),
					HealthyThreshold:   aws.Int64(3),
					UnhealthyThreshold: aws.Int64(3),
					Interval:           &duration60Seconds,
					Timeout:            &duration60Seconds,
					GracePeriod:        &duration15Seconds,
				}),
			},
			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/road/to/nowhere",
				Port:               "8080",
				SuccessCodes:       "200-299",
				HealthyThreshold:   aws.Int64(3),
				UnhealthyThreshold: aws.Int64(3),
				Interval:           aws.Int64(60),
				Timeout:            aws.Int64(60),
				GracePeriod:        15,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wantedOpts, convertHTTPHealthCheck(&tc.input))
		})
	}
}

func Test_convertManagedFSInfo(t *testing.T) {
	testCases := map[string]struct {
		inVolumes         map[string]*manifest.Volume
		wantManagedConfig *template.ManagedVolumeCreationInfo
		wantVolumes       map[string]manifest.Volume
	}{
		"no managed config": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: nil,
			wantVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
		},
		"with managed config": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: &template.ManagedVolumeCreationInfo{
				Name:    aws.String("wordpress"),
				DirName: aws.String("fe"),
				UID:     aws.Uint32(1336298249),
				GID:     aws.Uint32(1336298249),
			},
			wantVolumes: map[string]manifest.Volume{},
		},
		"with custom UID": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(10000),
							GID: aws.Uint32(100000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantManagedConfig: &template.ManagedVolumeCreationInfo{
				Name:    aws.String("wordpress"),
				DirName: aws.String("fe"),
				UID:     aws.Uint32(10000),
				GID:     aws.Uint32(100000),
			},
			wantVolumes: map[string]manifest.Volume{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			gotManaged := convertManagedFSInfo(aws.String("fe"), tc.inVolumes)

			// THEN
			require.Equal(t, tc.wantManagedConfig, gotManaged)
		})
	}
}
func Test_convertStorageOpts(t *testing.T) {
	testCases := map[string]struct {
		inVolumes   map[string]*manifest.Volume
		inEphemeral *int
		wantOpts    template.StorageOpts
	}{
		"minimal configuration": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("DISABLED"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        false,
					},
				},
			},
		},
		"empty volume for shareable storage between sidecar and main container": {
			inVolumes: map[string]*manifest.Volume{
				"scratch": {
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/scratch"),
					},
					EFS: manifest.EFSConfigOrBool{
						Enabled: aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("scratch"),
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/scratch"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("scratch"),
					},
				},
			},
		},
		"full specification with access point renders correctly": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							AuthConfig: manifest.AuthorizationConfig{
								IAM:           aws.Bool(true),
								AccessPointID: aws.String("ap-1234"),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("ENABLED"),
							AccessPointID: aws.String("ap-1234"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID:  aws.String("fs-1234"),
						AccessPointID: aws.String("ap-1234"),
						Write:         true,
					},
				},
			},
		},
		"full specification without access point renders correctly": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							AuthConfig: manifest.AuthorizationConfig{
								IAM: aws.Bool(true),
							},
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							IAM:           aws.String("ENABLED"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        true,
					},
				},
			},
		},
		"managed EFS": {
			inVolumes: map[string]*manifest.Volume{
				"efs": {
					EFS: manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1336298249),
					GID:     aws.Uint32(1336298249),
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
				},
			},
		},
		"managed EFS with config": {
			inVolumes: map[string]*manifest.Volume{
				"efs": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							UID: aws.Uint32(1000),
							GID: aws.Uint32(10000),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1000),
					GID:     aws.Uint32(10000),
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
				},
			},
		},
		"managed EFS and BYO": {
			inVolumes: map[string]*manifest.Volume{
				"efs": {
					EFS: manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
				"otherefs": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/stuff"),
						ReadOnly:      aws.Bool(false),
					},
				},
				"ephemeral": {
					EFS: manifest.EFSConfigOrBool{},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/ephemeral"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantOpts: template.StorageOpts{
				ManagedVolumeInfo: &template.ManagedVolumeCreationInfo{
					Name:    aws.String("efs"),
					DirName: aws.String("fe"),
					UID:     aws.Uint32(1336298249),
					GID:     aws.Uint32(1336298249),
				},
				Volumes: []*template.Volume{
					{
						Name: aws.String("otherefs"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("DISABLED"),
						},
					},
					{
						Name: aws.String("ephemeral"),
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("efs"),
					},
					{
						ContainerPath: aws.String("/var/stuff"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("otherefs"),
					},
					{
						ContainerPath: aws.String("/var/ephemeral"),
						ReadOnly:      aws.Bool(false),
						SourceVolume:  aws.String("ephemeral"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        true,
					},
				},
			},
		},
		"efs specified with just ID": {
			inVolumes: map[string]*manifest.Volume{
				"wordpress": {
					EFS: manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
			},
			wantOpts: template.StorageOpts{
				Volumes: []*template.Volume{
					{
						Name: aws.String("wordpress"),
						EFS: &template.EFSVolumeConfiguration{
							Filesystem:    aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							IAM:           aws.String("DISABLED"),
						},
					},
				},
				MountPoints: []*template.MountPoint{
					{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
						SourceVolume:  aws.String("wordpress"),
					},
				},
				EFSPerms: []*template.EFSPermission{
					{
						FilesystemID: aws.String("fs-1234"),
						Write:        false,
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			s := manifest.Storage{
				Volumes:   tc.inVolumes,
				Ephemeral: tc.inEphemeral,
			}

			// WHEN
			got := convertStorageOpts(aws.String("fe"), s)

			// THEN
			require.ElementsMatch(t, tc.wantOpts.EFSPerms, got.EFSPerms)
			require.ElementsMatch(t, tc.wantOpts.MountPoints, got.MountPoints)
			require.ElementsMatch(t, tc.wantOpts.Volumes, got.Volumes)
			require.Equal(t, tc.wantOpts.ManagedVolumeInfo, got.ManagedVolumeInfo)
		})
	}
}

func Test_convertExecuteCommand(t *testing.T) {
	testCases := map[string]struct {
		inConfig manifest.ExecuteCommand

		wanted *template.ExecuteCommandOpts
	}{
		"without exec enabled": {
			inConfig: manifest.ExecuteCommand{},
			wanted:   nil,
		},
		"exec enabled": {
			inConfig: manifest.ExecuteCommand{
				Enable: aws.Bool(true),
			},
			wanted: &template.ExecuteCommandOpts{},
		},
		"exec enabled with config": {
			inConfig: manifest.ExecuteCommand{
				Config: manifest.ExecuteCommandConfig{
					Enable: aws.Bool(true),
				},
			},
			wanted: &template.ExecuteCommandOpts{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			exec := tc.inConfig
			got := convertExecuteCommand(&exec)

			require.Equal(t, tc.wanted, got)
		})
	}
}

func Test_convertSidecarMountPoints(t *testing.T) {
	testCases := map[string]struct {
		inMountPoints  []manifest.SidecarMountPoint
		wantMountPoint []*template.MountPoint
	}{
		"fully specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantMountPoint: []*template.MountPoint{
				{
					ContainerPath: aws.String("/var/www/wp-content"),
					ReadOnly:      aws.Bool(false),
					SourceVolume:  aws.String("wordpress"),
				},
			},
		},
		"readonly defaults to true": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
					},
				},
			},
			wantMountPoint: []*template.MountPoint{
				{
					ContainerPath: aws.String("/var/www/wp-content"),
					ReadOnly:      aws.Bool(true),
					SourceVolume:  aws.String("wordpress"),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := convertSidecarMountPoints(tc.inMountPoints)
			require.Equal(t, tc.wantMountPoint, got)
		})
	}
}

func Test_convertEphemeral(t *testing.T) {
	testCases := map[string]struct {
		inEphemeral *int

		wanted      *int
		wantedError error
	}{
		"without storage enabled": {
			inEphemeral: nil,
			wanted:      nil,
		},
		"ephemeral specified correctly": {
			inEphemeral: aws.Int(100),
			wanted:      aws.Int(100),
		},
		"ephemeral specified at 20 GiB": {
			inEphemeral: aws.Int(20),
			wanted:      nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := convertEphemeral(tc.inEphemeral)
			require.Equal(t, got, tc.wanted)
		})
	}
}

func Test_convertPublish(t *testing.T) {
	accountId := "123456789123"
	partition := "aws"
	region := "us-west-2"
	app := "testapp"
	env := "testenv"
	svc := "hello"
	testCases := map[string]struct {
		inTopics []manifest.Topic

		wanted      *template.PublishOpts
		wantedError error
	}{
		"no manifest publishers should return nil": {
			inTopics: nil,
			wanted:   nil,
		},
		"empty manifest publishers should return nil": {
			inTopics: []manifest.Topic{},
			wanted:   nil,
		},
		"valid publish": {
			inTopics: []manifest.Topic{
				{
					Name: aws.String("topic1"),
				},
				{
					Name: aws.String("topic2"),
				},
			},
			wanted: &template.PublishOpts{
				Topics: []*template.Topic{
					{
						Name:      aws.String("topic1"),
						AccountID: accountId,
						Partition: partition,
						Region:    region,
						App:       app,
						Env:       env,
						Svc:       svc,
					},
					{

						Name:      aws.String("topic2"),
						AccountID: accountId,
						Partition: partition,
						Region:    region,
						App:       app,
						Env:       env,
						Svc:       svc,
					},
				},
			},
		},
		"valid publish with fifo enabled and standard topics": {
			inTopics: []manifest.Topic{
				{
					Name: aws.String("topic1.fifo"),
					FIFO: manifest.FIFOTopicAdvanceConfigOrBool{Enable: aws.Bool(true)},
				},
				{
					Name: aws.String("topic2"),
				},
			},
			wanted: &template.PublishOpts{
				Topics: []*template.Topic{
					{
						Name:            aws.String("topic1.fifo"),
						FIFOTopicConfig: &template.FIFOTopicConfig{},
						AccountID:       accountId,
						Partition:       partition,
						Region:          region,
						App:             app,
						Env:             env,
						Svc:             svc,
					},
					{

						Name:            aws.String("topic2"),
						FIFOTopicConfig: nil,
						AccountID:       accountId,
						Partition:       partition,
						Region:          region,
						App:             app,
						Env:             env,
						Svc:             svc,
					},
				},
			},
		},
		"valid publish with advanced fifo and standard topics": {
			inTopics: []manifest.Topic{
				{
					Name: aws.String("topic1.fifo"),
					FIFO: manifest.FIFOTopicAdvanceConfigOrBool{
						Advanced: manifest.FIFOTopicAdvanceConfig{
							ContentBasedDeduplication: aws.Bool(true),
						},
					},
				},
				{
					Name: aws.String("topic2"),
				},
			},
			wanted: &template.PublishOpts{
				Topics: []*template.Topic{
					{
						Name: aws.String("topic1.fifo"),
						FIFOTopicConfig: &template.FIFOTopicConfig{
							ContentBasedDeduplication: aws.Bool(true),
						},
						AccountID: accountId,
						Partition: partition,
						Region:    region,
						App:       app,
						Env:       env,
						Svc:       svc,
					},
					{

						Name:      aws.String("topic2"),
						AccountID: accountId,
						Partition: partition,
						Region:    region,
						App:       app,
						Env:       env,
						Svc:       svc,
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := convertPublish(tc.inTopics, accountId, region, app, env, svc)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Equal(t, got, tc.wanted)
			}
		})
	}
}

func Test_convertSubscribe(t *testing.T) {
	duration111Seconds := 111 * time.Second
	mockStruct := map[string]interface{}{
		"store": []string{"example_corp"},
	}
	testCases := map[string]struct {
		inSubscribe *manifest.WorkerService

		wanted *template.SubscribeOpts
	}{
		"empty subscription": { //1
			inSubscribe: &manifest.WorkerService{},
			wanted:      nil,
		},
		"valid subscribe": { //2
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{
							Retention: &duration111Seconds,
							Delay:     &duration111Seconds,
							Timeout:   &duration111Seconds,
							DeadLetter: manifest.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
						},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
					},
				},
				Queue: &template.SQSQueue{
					Retention: aws.Int64(111),
					Delay:     aws.Int64(111),
					Timeout:   aws.Int64(111),
					DeadLetter: &template.DeadLetterQueue{
						Tries: aws.Uint16(35),
					},
				},
			},
		},
		"valid subscribe with default queue configs": { //3
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with queue enabled": { //4
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Enabled: aws.Bool(true),
								},
								FilterPolicy: mockStruct,
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:         aws.String("name"),
						Service:      aws.String("svc"),
						Queue:        &template.SQSQueue{},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with minimal queue": { //5
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
									},
								},
								FilterPolicy: mockStruct,
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with high throughput fifo sqs": { //6
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
										FIFO: manifest.FIFOAdvanceConfigOrBool{Advanced: manifest.FIFOAdvanceConfig{HighThroughputFifo: aws.Bool(true)}},
									},
								},
								FilterPolicy: mockStruct,
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
							FIFOQueueConfig: &template.FIFOQueueConfig{
								FIFOThroughputLimit: aws.String("perMessageGroupId"),
								DeduplicationScope:  aws.String("messageGroup"),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with custom minimal fifo sqs config values": {
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
										FIFO: manifest.FIFOAdvanceConfigOrBool{Enable: aws.Bool(true)},
									},
								},
								FilterPolicy: mockStruct,
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
							FIFOQueueConfig: &template.FIFOQueueConfig{},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with custom complete fifo sqs config values": { //7
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
										FIFO: manifest.FIFOAdvanceConfigOrBool{
											Advanced: manifest.FIFOAdvanceConfig{
												FIFOThroughputLimit:       aws.String("queue"),
												DeduplicationScope:        aws.String("perQueue"),
												ContentBasedDeduplication: aws.Bool(true),
											},
										},
									},
								},
								FilterPolicy: mockStruct,
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
							FIFOQueueConfig: &template.FIFOQueueConfig{
								FIFOThroughputLimit:       aws.String("queue"),
								DeduplicationScope:        aws.String("perQueue"),
								ContentBasedDeduplication: aws.Bool(true),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with custom complete fifo sqs config and standard topic subscription to a default standard queue": { //8
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
										FIFO: manifest.FIFOAdvanceConfigOrBool{
											Advanced: manifest.FIFOAdvanceConfig{
												FIFOThroughputLimit:       aws.String("queue"),
												DeduplicationScope:        aws.String("perQueue"),
												ContentBasedDeduplication: aws.Bool(true),
											},
										},
									},
								},
								FilterPolicy: mockStruct,
							},
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
							FIFOQueueConfig: &template.FIFOQueueConfig{
								FIFOThroughputLimit:       aws.String("queue"),
								DeduplicationScope:        aws.String("perQueue"),
								ContentBasedDeduplication: aws.Bool(true),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with custom complete fifo sqs config and multiple standard topic subscriptions to a default standard queue": { //9
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
										FIFO: manifest.FIFOAdvanceConfigOrBool{
											Advanced: manifest.FIFOAdvanceConfig{
												FIFOThroughputLimit:       aws.String("queue"),
												DeduplicationScope:        aws.String("perQueue"),
												ContentBasedDeduplication: aws.Bool(true),
											},
										},
									},
								},
								FilterPolicy: mockStruct,
							},
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
							{
								Name:    aws.String("name1"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
							FIFOQueueConfig: &template.FIFOQueueConfig{
								FIFOThroughputLimit:       aws.String("queue"),
								DeduplicationScope:        aws.String("perQueue"),
								ContentBasedDeduplication: aws.Bool(true),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
					},
					{
						Name:    aws.String("name1"),
						Service: aws.String("svc"),
					},
				},
				Queue: nil,
			},
		},
		"valid subscribe with standard sqs config and fifo topic subscription to a default fifo queue": { //10
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
									},
								},
								FilterPolicy: mockStruct,
							},
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{
							FIFO: manifest.FIFOAdvanceConfigOrBool{Enable: aws.Bool(true)},
						},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
					},
				},
				Queue: &template.SQSQueue{
					FIFOQueueConfig: &template.FIFOQueueConfig{},
				},
			},
		},
		"valid subscribe with standard sqs config and multiple fifo topic subscriptions to a default fifo queue": { //11
			inSubscribe: &manifest.WorkerService{
				WorkerServiceConfig: manifest.WorkerServiceConfig{
					Subscribe: manifest.SubscribeConfig{
						Topics: []manifest.TopicSubscription{
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
								Queue: manifest.SQSQueueOrBool{
									Advanced: manifest.SQSQueue{
										Retention: &duration111Seconds,
										Delay:     &duration111Seconds,
										Timeout:   &duration111Seconds,
										DeadLetter: manifest.DeadLetterQueue{
											Tries: aws.Uint16(35),
										},
									},
								},
								FilterPolicy: mockStruct,
							},
							{
								Name:    aws.String("name"),
								Service: aws.String("svc"),
							},
							{
								Name:    aws.String("name1"),
								Service: aws.String("svc"),
							},
						},
						Queue: manifest.SQSQueue{
							FIFO: manifest.FIFOAdvanceConfigOrBool{Enable: aws.Bool(true)},
						},
					},
				},
			},
			wanted: &template.SubscribeOpts{
				Topics: []*template.TopicSubscription{
					{
						Name:    aws.String("name"),
						Service: aws.String("svc"),
						Queue: &template.SQSQueue{
							Retention: aws.Int64(111),
							Delay:     aws.Int64(111),
							Timeout:   aws.Int64(111),
							DeadLetter: &template.DeadLetterQueue{
								Tries: aws.Uint16(35),
							},
						},
						FilterPolicy: aws.String(`{"store":["example_corp"]}`),
					},
					{
						Name:    aws.String("name.fifo"),
						Service: aws.String("svc"),
					},
					{
						Name:    aws.String("name1.fifo"),
						Service: aws.String("svc"),
					},
				},
				Queue: &template.SQSQueue{
					FIFOQueueConfig: &template.FIFOQueueConfig{},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := convertSubscribe(tc.inSubscribe)
			require.Equal(t, tc.wanted, got)
			require.NoError(t, err)
		})
	}
}

func Test_convertPlatform(t *testing.T) {
	testCases := map[string]struct {
		in  manifest.PlatformArgsOrString
		out template.RuntimePlatformOpts
	}{
		"should return empty struct if user did not set a platform field in the manifest": {},
		"should return windows server 2019 full and x86_64 when advanced config specifies 2019 full": {
			in: manifest.PlatformArgsOrString{
				PlatformArgs: manifest.PlatformArgs{
					OSFamily: aws.String(manifest.OSWindowsServer2019Full),
					Arch:     aws.String(manifest.ArchX86),
				},
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSWindowsServer2019Full,
				Arch: template.ArchX86,
			},
		},
		"should return windows server 2019 core and x86_64 when advanced config specifies 2019 core": {
			in: manifest.PlatformArgsOrString{
				PlatformArgs: manifest.PlatformArgs{
					OSFamily: aws.String(manifest.OSWindowsServer2019Core),
					Arch:     aws.String(manifest.ArchX86),
				},
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSWindowsServer2019Core,
				Arch: template.ArchX86,
			},
		},
		"should return windows server 2022 full and x86_64 when advanced config specifies 2022 full": {
			in: manifest.PlatformArgsOrString{
				PlatformArgs: manifest.PlatformArgs{
					OSFamily: aws.String(manifest.OSWindowsServer2022Full),
					Arch:     aws.String(manifest.ArchX86),
				},
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSWindowsServer2022Full,
				Arch: template.ArchX86,
			},
		},
		"should return windows server 2022 core and x86_64 when advanced config specifies 2022 core": {
			in: manifest.PlatformArgsOrString{
				PlatformArgs: manifest.PlatformArgs{
					OSFamily: aws.String(manifest.OSWindowsServer2022Core),
					Arch:     aws.String(manifest.ArchX86),
				},
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSWindowsServer2022Core,
				Arch: template.ArchX86,
			},
		},
		"should return windows server core 2019 and x86_64 when platform is 'windows/x86_64'": {
			in: manifest.PlatformArgsOrString{
				PlatformString: (*manifest.PlatformString)(aws.String("windows/amd64")),
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSWindowsServer2019Core,
				Arch: template.ArchX86,
			},
		},
		"should return linux and x86_64 when platform is 'linux/amd64'": {
			in: manifest.PlatformArgsOrString{
				PlatformString: (*manifest.PlatformString)(aws.String("linux/amd64")),
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSLinux,
				Arch: template.ArchX86,
			},
		},
		"should return linux and arm when platform is 'linux/arm'": {
			in: manifest.PlatformArgsOrString{
				PlatformString: (*manifest.PlatformString)(aws.String("linux/arm")),
			},
			out: template.RuntimePlatformOpts{
				OS:   template.OSLinux,
				Arch: template.ArchARM64,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.out, convertPlatform(tc.in))
		})
	}
}

func Test_convertHTTPVersion(t *testing.T) {
	testCases := map[string]struct {
		in     *string
		wanted *string
	}{
		"should return nil if there is no user input": {},
		"should return as uppercase on any user input": {
			in:     aws.String("gRPC"),
			wanted: aws.String("GRPC"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, convertHTTPVersion(tc.in))
		})
	}
}

func Test_convertCustomResources(t *testing.T) {
	testCases := map[string]struct {
		in        map[string]string
		wanted    map[string]template.S3ObjectLocation
		wantedErr error
	}{
		"returns a wrapped error if a url cannot be parsed": {
			in: map[string]string{
				"EnvControllerFunction":       "https://my-bucket.s3.us-west-2.amazonaws.com/puppy.png",
				"DynamicDesiredCountFunction": "bad",
			},
			wantedErr: errors.New(`convert custom resource "DynamicDesiredCountFunction" url: cannot parse S3 URL bad into bucket name and key`),
		},
		"transforms custom resources with valid urls": {
			in: map[string]string{
				"EnvControllerFunction": "https://my-bucket.s3.us-west-2.amazonaws.com/good/dogs/puppy.png",
			},
			wanted: map[string]template.S3ObjectLocation{
				"EnvControllerFunction": {
					Bucket: "my-bucket",
					Key:    "good/dogs/puppy.png",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			out, err := convertCustomResources(tc.in)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, out)
			}
		})
	}
}
