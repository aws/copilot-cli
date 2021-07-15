// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/stretchr/testify/require"
)

func Test_convertSidecar(t *testing.T) {
	mockImage := aws.String("mockImage")
	mockWorkloadName := "frontend"
	mockMap := map[string]string{"foo": "bar"}
	mockCredsParam := aws.String("mockCredsParam")
	circularDependencyErr := fmt.Errorf("circular container dependency chain includes the following containers: ")
	testCases := map[string]struct {
		inPort            *string
		inEssential       bool
		inLabels          map[string]string
		inDependsOn       map[string]string
		inImg             manifest.Image
		inImageOverride   manifest.ImageOverride
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
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"good port with protocol": {
			inPort:      aws.String("2000/udp"),
			inEssential: true,

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				Protocol:   aws.String("udp"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
			},
		},
		"invalid container dependency due to circularly depending on itself": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"foo": "start",
			},

			wantedErr: fmt.Errorf("container foo cannot depend on itself"),
		},
		"invalid container dependency due to circularly depending on another container": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"frontend": "start",
			},
			inImg: manifest.Image{
				DependsOn: map[string]string{
					"foo": "start",
				},
			},
			wantedErr:         circularDependencyErr,
			circDepContainers: []string{"frontend", "foo"},
		},
		"invalid container dependency status": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"frontend": "never",
			},
			wantedErr: errInvalidDependsOnStatus,
		},
		"invalid essential container dependency status": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"frontend": "complete",
			},
			wantedErr: errEssentialContainerStatus,
		},
		"good essential container dependencies": {
			inPort:      aws.String("2000"),
			inEssential: true,
			inDependsOn: map[string]string{
				"frontend": "start",
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(true),
				DependsOn: map[string]string{
					"frontend": "START",
				},
			},
		},
		"good nonessential container dependencies": {
			inEssential: false,
			inDependsOn: map[string]string{
				"frontend": "start",
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
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
				Name:       aws.String("foo"),
				Port:       aws.String("2000"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				DockerLabels: map[string]string{
					"com.amazonaws.ecs.copilot.sidecar.description": "wow",
				},
			},
		},
		"do not specify image override": {
			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    nil,
			},
		},
		"specify entrypoint as a string": {
			inImageOverride: manifest.ImageOverride{
				EntryPoint: &manifest.EntryPointOverride{String: aws.String("bin")},
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: []string{"bin"},
				Command:    nil,
			},
		},
		"specify entrypoint as a string slice": {
			inImageOverride: manifest.ImageOverride{
				EntryPoint: &manifest.EntryPointOverride{StringSlice: []string{"bin", "arg"}},
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: []string{"bin", "arg"},
				Command:    nil,
			},
		},
		"specify command as a string": {
			inImageOverride: manifest.ImageOverride{
				Command: &manifest.CommandOverride{String: aws.String("arg")},
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    []string{"arg"},
			},
		},
		"specify command as a string slice": {
			inImageOverride: manifest.ImageOverride{
				Command: &manifest.CommandOverride{StringSlice: []string{"arg1", "arg2"}},
			},

			wanted: &template.SidecarOpts{
				Name:       aws.String("foo"),
				CredsParam: mockCredsParam,
				Image:      mockImage,
				Secrets:    mockMap,
				Variables:  mockMap,
				Essential:  aws.Bool(false),
				EntryPoint: nil,
				Command:    []string{"arg1", "arg2"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sidecar := map[string]*manifest.SidecarConfig{
				"foo": {
					CredsParam:    mockCredsParam,
					Image:         mockImage,
					Secrets:       mockMap,
					Variables:     mockMap,
					Essential:     aws.Bool(tc.inEssential),
					Port:          tc.inPort,
					DockerLabels:  tc.inLabels,
					DependsOn:     tc.inDependsOn,
					ImageOverride: tc.inImageOverride,
				},
			}
			got, err := convertSidecar(convertSidecarOpts{
				sidecarConfig: sidecar,
				imageConfig:   &tc.inImg,
				workloadName:  mockWorkloadName,
			})

			if tc.wantedErr == circularDependencyErr {
				require.Contains(t, err.Error(), circularDependencyErr.Error())
				for _, container := range tc.circDepContainers {
					require.Contains(t, err.Error(), container)
				}
			} else if tc.wantedErr != nil {
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
	testCases := map[string]struct {
		input       *manifest.AdvancedCount
		expected    *template.AdvancedCount
		expectedErr error
	}{
		"returns nil if nil": {
			input:    nil,
			expected: nil,
		},
		"returns nil if empty": {
			input:    &manifest.AdvancedCount{},
			expected: nil,
		},
		"success with spot count": {
			input: &manifest.AdvancedCount{
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
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					Value: &mockRange,
				},
				CPU: aws.Int(70),
			},
			expected: &template.AdvancedCount{
				Autoscaling: &template.AutoscalingOpts{
					MinCapacity: aws.Int(1),
					MaxCapacity: aws.Int(10),
					CPU:         aws.Float64(70),
				},
			},
		},
		"success with spot autoscaling": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(2),
						Max:      aws.Int(20),
						SpotFrom: aws.Int(5),
					},
				},
				CPU: aws.Int(70),
			},
			expected: &template.AdvancedCount{
				Autoscaling: &template.AutoscalingOpts{
					MinCapacity: aws.Int(2),
					MaxCapacity: aws.Int(20),
					CPU:         aws.Float64(70),
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
		input       *manifest.AdvancedCount
		expected    []*template.CapacityProviderStrategy
		expectedErr error
	}{
		"with spot as desiredCount": {
			input: &manifest.AdvancedCount{
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
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
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
			},
		},
		"with scaling into spot": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
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
		"returns nil if no spot config specified": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					Value: &mockRange,
				},
			},
			expected: nil,
		},
		"errors if spot specified with range": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					Value: &mockRange,
				},
				Spot: aws.Int(3),
			},
			expectedErr: errInvalidSpotConfig,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual, err := convertCapacityProviders(tc.input)

			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}

func Test_convertAutoscaling(t *testing.T) {
	mockRange := manifest.IntRangeBand("1-100")
	badRange := manifest.IntRangeBand("badRange")
	mockRequests := 1000
	mockResponseTime := 512 * time.Millisecond
	testCases := map[string]struct {
		input *manifest.AdvancedCount

		wanted    *template.AutoscalingOpts
		wantedErr error
	}{
		"invalid range": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					Value: &badRange,
				},
			},

			wantedErr: fmt.Errorf("invalid range value badRange. Should be in format of ${min}-${max}"),
		},
		"success": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					Value: &mockRange,
				},
				CPU:          aws.Int(70),
				Memory:       aws.Int(80),
				Requests:     aws.Int(mockRequests),
				ResponseTime: &mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(100),
				MinCapacity:  aws.Int(1),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"success with range subfields": {
			input: &manifest.AdvancedCount{
				Range: &manifest.Range{
					RangeConfig: manifest.RangeConfig{
						Min:      aws.Int(5),
						Max:      aws.Int(10),
						SpotFrom: aws.Int(5),
					},
				},
				CPU:          aws.Int(70),
				Memory:       aws.Int(80),
				Requests:     aws.Int(mockRequests),
				ResponseTime: &mockResponseTime,
			},

			wanted: &template.AutoscalingOpts{
				MaxCapacity:  aws.Int(10),
				MinCapacity:  aws.Int(5),
				CPU:          aws.Float64(70),
				Memory:       aws.Float64(80),
				Requests:     aws.Float64(1000),
				ResponseTime: aws.Float64(0.512),
			},
		},
		"returns nil if spot specified": {
			input: &manifest.AdvancedCount{
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

func Test_convertHTTPHealthCheck(t *testing.T) {
	// These are used by reference to represent the output of the manifest.durationp function.
	duration15Seconds := time.Duration(15 * time.Second)
	duration60Seconds := time.Duration(60 * time.Second)
	testCases := map[string]struct {
		inputPath               *string
		inputSuccessCodes       *string
		inputHealthyThreshold   *int64
		inputUnhealthyThreshold *int64
		inputInterval           *time.Duration
		inputTimeout            *time.Duration

		wantedOpts template.HTTPHealthCheckOpts
	}{
		"no fields indicated in manifest": {
			inputPath:               nil,
			inputSuccessCodes:       nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
			},
		},
		"just HealthyThreshold": {
			inputPath:               nil,
			inputSuccessCodes:       nil,
			inputHealthyThreshold:   aws.Int64(5),
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:  "/",
				HealthyThreshold: aws.Int64(5),
			},
		},
		"just UnhealthyThreshold": {
			inputPath:               nil,
			inputSuccessCodes:       nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: aws.Int64(5),
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/",
				UnhealthyThreshold: aws.Int64(5),
			},
		},
		"just Interval": {
			inputPath:               nil,
			inputSuccessCodes:       nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           &duration15Seconds,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Interval:        aws.Int64(15),
			},
		},
		"just Timeout": {
			inputPath:               nil,
			inputSuccessCodes:       nil,
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            &duration15Seconds,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				Timeout:         aws.Int64(15),
			},
		},
		"just SuccessCodes": {
			inputPath:               nil,
			inputSuccessCodes:       aws.String("200,301"),
			inputHealthyThreshold:   nil,
			inputUnhealthyThreshold: nil,
			inputInterval:           nil,
			inputTimeout:            nil,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath: "/",
				SuccessCodes:    "200,301",
			},
		},
		"all values changed in manifest": {
			inputPath:               aws.String("/road/to/nowhere"),
			inputSuccessCodes:       aws.String("200-299"),
			inputHealthyThreshold:   aws.Int64(3),
			inputUnhealthyThreshold: aws.Int64(3),
			inputInterval:           &duration60Seconds,
			inputTimeout:            &duration60Seconds,

			wantedOpts: template.HTTPHealthCheckOpts{
				HealthCheckPath:    "/road/to/nowhere",
				SuccessCodes:       "200-299",
				HealthyThreshold:   aws.Int64(3),
				UnhealthyThreshold: aws.Int64(3),
				Interval:           aws.Int64(60),
				Timeout:            aws.Int64(60),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			hc := manifest.HealthCheckArgsOrString{
				HealthCheckPath: tc.inputPath,
				HealthCheckArgs: manifest.HTTPHealthCheckArgs{
					Path:               tc.inputPath,
					SuccessCodes:       tc.inputSuccessCodes,
					HealthyThreshold:   tc.inputHealthyThreshold,
					UnhealthyThreshold: tc.inputUnhealthyThreshold,
					Timeout:            tc.inputTimeout,
					Interval:           tc.inputInterval,
				},
			}
			// WHEN
			actualOpts := convertHTTPHealthCheck(&hc)

			// THEN
			require.Equal(t, tc.wantedOpts, actualOpts)
		})
	}
}

func Test_convertManagedFSInfo(t *testing.T) {
	testCases := map[string]struct {
		inVolumes         map[string]manifest.Volume
		wantManagedConfig *template.ManagedVolumeCreationInfo
		wantVolumes       map[string]manifest.Volume
		wantErr           string
	}{
		"no managed config": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
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
					EFS: &manifest.EFSConfigOrBool{
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
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
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
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
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
			gotManaged, err := convertManagedFSInfo(aws.String("fe"), tc.inVolumes)

			// THEN
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantManagedConfig, gotManaged)
			}
		})
	}
}
func Test_convertStorageOpts(t *testing.T) {
	testCases := map[string]struct {
		inVolumes   map[string]manifest.Volume
		inEphemeral *int
		wantOpts    template.StorageOpts
		wantErr     string
	}{
		"minimal configuration": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
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
			inVolumes: map[string]manifest.Volume{
				"scratch": {
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/scratch"),
					},
					EFS: &manifest.EFSConfigOrBool{
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
		"container path not specified": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID: aws.String("fs-1234"),
						},
					},
				},
			},
			wantErr: fmt.Sprintf("validate container configuration for volume wordpress: %s", errNoContainerPath.Error()),
		},
		"full specification with access point renders correctly": {
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/"),
							AuthConfig: &manifest.AuthorizationConfig{
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
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Advanced: manifest.EFSVolumeConfiguration{
							FileSystemID:  aws.String("fs-1234"),
							RootDirectory: aws.String("/wordpress"),
							AuthConfig: &manifest.AuthorizationConfig{
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
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
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
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
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
		"error when multiple managed volumes specified": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/abc"),
					},
				},
			},
			wantErr: "cannot specify more than one managed volume per service",
		},
		"managed EFS and BYO": {
			inVolumes: map[string]manifest.Volume{
				"efs": {
					EFS: &manifest.EFSConfigOrBool{
						Enabled: aws.Bool(true),
					},
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www"),
						ReadOnly:      aws.Bool(true),
					},
				},
				"otherefs": {
					EFS: &manifest.EFSConfigOrBool{
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
					EFS: nil,
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
			inVolumes: map[string]manifest.Volume{
				"wordpress": {
					EFS: &manifest.EFSConfigOrBool{
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
			got, err := convertStorageOpts(aws.String("fe"), &s)

			// THEN
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t, tc.wantOpts.EFSPerms, got.EFSPerms)
				require.ElementsMatch(t, tc.wantOpts.MountPoints, got.MountPoints)
				require.ElementsMatch(t, tc.wantOpts.Volumes, got.Volumes)
				require.Equal(t, tc.wantOpts.ManagedVolumeInfo, got.ManagedVolumeInfo)
			}
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
		wantErr        string
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
		"error when source not specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					MountPointOpts: manifest.MountPointOpts{
						ContainerPath: aws.String("/var/www/wp-content"),
						ReadOnly:      aws.Bool(false),
					},
				},
			},
			wantErr: errNoSourceVolume.Error(),
		},
		"error when path not specified": {
			inMountPoints: []manifest.SidecarMountPoint{
				{
					SourceVolume: aws.String("wordpress"),
					MountPointOpts: manifest.MountPointOpts{
						ReadOnly: aws.Bool(false),
					},
				},
			},
			wantErr: errNoContainerPath.Error(),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateSidecarMountPoints(tc.inMountPoints)
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				got := convertSidecarMountPoints(tc.inMountPoints)
				require.Equal(t, tc.wantMountPoint, got)
			}
		})
	}
}

func Test_validatePaths(t *testing.T) {
	t.Run("containerPath should be properly validated", func(t *testing.T) {
		require.NoError(t, validateContainerPath("/abc/90_"), "contains underscore")
		require.EqualError(t, validateContainerPath("/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "path must be less than 242 bytes in length", "too long")
		require.EqualError(t, validateContainerPath("/etc /bin/sh cat `i'm evil` > /dev/null"), "paths can only contain the characters a-zA-Z0-9.-_/", "invalid characters disallowed")
	})
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
		"ephemeral errors when size is too big": {
			inEphemeral: aws.Int(25000),
			wantedError: errEphemeralBadSize,
		},
		"ephemeral errors when size is too small": {
			inEphemeral: aws.Int(10),
			wantedError: errEphemeralBadSize,
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
			got, err := convertEphemeral(tc.inEphemeral)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Equal(t, got, tc.wanted)
			}
		})
	}
}

func Test_convertImageDependsOn(t *testing.T) {
	mockWorkloadName := "frontend"
	circularDependencyErr := fmt.Errorf("circular container dependency chain includes the following containers: ")
	testCases := map[string]struct {
		inImage           *manifest.Image
		inSidecars        map[string]*manifest.SidecarConfig
		circDepContainers []string

		wanted      map[string]string
		wantedError error
	}{
		"no container dependencies": {
			inImage: &manifest.Image{},
			wanted:  nil,
		},
		"invalid container dependency due to circular dependency on itself": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"frontend": "start",
				},
			},
			wantedError: fmt.Errorf("container frontend cannot depend on itself"),
		},
		"invalid container dependency due to circular dependency on a sidecar": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "start",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"sidecar2": "start",
					},
				},
				"sidecar2": {
					DependsOn: map[string]string{
						"frontend": "start",
					},
				},
			},
			wantedError:       circularDependencyErr,
			circDepContainers: []string{"frontend", "sidecar", "sidecar2"},
		},
		"invalid container dependency due to status": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "end",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					Essential: aws.Bool(false),
				},
			},
			wantedError: errInvalidSidecarDependsOnStatus,
		},
		"invalid implied essential container depdendency": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "complete",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantedError: errEssentialSidecarStatus,
		},
		"invalid set essential container depdendency": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "complete",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					Essential: aws.Bool(true),
				},
			},
			wantedError: errEssentialSidecarStatus,
		},
		"good essential container dependency": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "start",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wanted: map[string]string{
				"sidecar": "START",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := convertImageDependsOn(convertSidecarOpts{
				sidecarConfig: tc.inSidecars,
				imageConfig:   tc.inImage,
				workloadName:  mockWorkloadName,
			})
			if tc.wantedError == circularDependencyErr {
				require.Contains(t, err.Error(), circularDependencyErr.Error())
				for _, container := range tc.circDepContainers {
					require.Contains(t, err.Error(), container)
				}
			} else if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Equal(t, got, tc.wanted)
			}
		})
	}
}
