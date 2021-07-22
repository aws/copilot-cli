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
	"github.com/stretchr/testify/require"
)

func Test_validateEFSConfig(t *testing.T) {
	testCases := map[string]struct {
		inConfig *manifest.EFSConfigOrBool

		wantErr error
	}{
		"no EFS config": {
			inConfig: nil,
			wantErr:  nil,
		},
		"managed EFS config": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(true),
			},
		},
		"EFS explicitly disabled": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(false),
			},
		},
		"advanced managed EFS config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
					GID: aws.Uint32(12345),
				},
			},
		},
		"BYO EFS": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID:  aws.String("fs-1234"),
					RootDirectory: aws.String("/files"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM: aws.Bool(true),
					},
				},
			},
		},
		"error when access point specified with root dir": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID:  aws.String("fs-1234"),
					RootDirectory: aws.String("/files"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM:           aws.Bool(true),
						AccessPointID: aws.String("fsap-12345"),
					},
				},
			},
			wantErr: errAccessPointWithRootDirectory,
		},
		"error when access point specified without IAM": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM:           aws.Bool(false),
						AccessPointID: aws.String("fsap-12345"),
					},
				},
			},
			wantErr: errAccessPointWithoutIAM,
		},
		"Enabled with advanced config": {
			inConfig: &manifest.EFSConfigOrBool{
				Enabled: aws.Bool(true),
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidEFSConfig,
		},
		"UID with BYO": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					FileSystemID: aws.String("fs-1234"),
					UID:          aws.Uint32(12345),
					GID:          aws.Uint32(12345),
				},
			},
			wantErr: errUIDWithNonManagedFS,
		},
		"invalid UID config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidUIDGIDConfig,
		},
		"invalid GID config": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errInvalidUIDGIDConfig,
		},
		"error when UID is 0": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					UID: aws.Uint32(0),
					GID: aws.Uint32(12345),
				},
			},
			wantErr: errReservedUID,
		},
		"empty EFS config should be invalid": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{},
			},
			wantErr: errEmptyEFSConfig,
		},
		"FSID not specified for BYO": {
			inConfig: &manifest.EFSConfigOrBool{
				Advanced: manifest.EFSVolumeConfiguration{
					RootDirectory: aws.String("/storage"),
					AuthConfig: &manifest.AuthorizationConfig{
						IAM: aws.Bool(true),
					},
				},
			},
			wantErr: errNoFSID,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			vol := manifest.Volume{
				EFS: tc.inConfig,
			}
			gotErr := validateEFSConfig(vol)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateSidecarDependsOn(t *testing.T) {
	mockSidecarName := "sidecar"
	mockWorkloadName := "frontend"
	testCases := map[string]struct {
		inSidecar   *manifest.SidecarConfig
		allSidecars map[string]*manifest.SidecarConfig

		wantErr error
	}{
		"no sidecar dependencies": {
			inSidecar: &manifest.SidecarConfig{},
			wantErr:   nil,
		},
		"working set essential sidecar with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar1": "START",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar1": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: nil,
		},
		"working implied essential container with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"frontend": "START",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"working non-essential sidecar with container dependency": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "COMPLETE",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(false),
				},
			},
			wantErr: nil,
		},
		"error when sidecar container dependency status is invalid": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "END",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(false),
				},
			},
			wantErr: errInvalidSidecarDependsOnStatus,
		},
		"error when container dependency status is invalid": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"frontend": "END",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errInvalidDependsOnStatus,
		},
		"error when set essential sidecar has a status besides start": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "COMPLETE",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
				"sidecar2": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: errEssentialSidecarStatus,
		},
		"error when implied essential sidecar has a status besides start": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"sidecar2": "COMPLETE",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar":  {},
				"sidecar2": {},
			},
			wantErr: errEssentialSidecarStatus,
		},
		"error when essential container dependency status is invalid": {
			inSidecar: &manifest.SidecarConfig{
				DependsOn: map[string]string{
					"frontend": "COMPLETE",
				},
			},
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errEssentialContainerStatus,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := convertSidecarOpts{
				sidecarConfig: tc.allSidecars,
				imageConfig:   &manifest.Image{},
				workloadName:  mockWorkloadName,
			}
			gotErr := validateSidecarDependsOn(*tc.inSidecar, mockSidecarName, s)
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateNoCircularDependencies(t *testing.T) {
	mockWorkloadName := "frontend"
	image := manifest.Image{}
	circularDependencyErr := fmt.Errorf("circular container dependency chain includes the following containers: ")
	testCases := map[string]struct {
		allSidecars       map[string]*manifest.SidecarConfig
		circDepContainers []string

		wantErr error
	}{
		"no sidecar dependencies": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"working sidecars with container dependency": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"frontend": "START",
					},
				},
			},
			wantErr: nil,
		},
		"working sidecars with complex container dependencies": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"secondCar": "START",
					},
				},
				"secondCar": {
					DependsOn: map[string]string{
						"thirdCar": "START",
					},
				},
				"thirdCar": {
					DependsOn: map[string]string{
						"fourthCar": "START",
					},
				},
				"fourthCar": {},
			},
			wantErr: nil,
		},
		"error when sidecar depends on itself": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"sidecar": "START",
					},
				},
			},
			wantErr: fmt.Errorf("container sidecar cannot depend on itself"),
		},
		"error when sidecars circularly depend on each other": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"frontend": "START",
					},
				},
				"frontend": {
					DependsOn: map[string]string{
						"sidecar": "START",
					},
				},
			},
			wantErr:           circularDependencyErr,
			circDepContainers: []string{"sidecar", "frontend"},
		},
		"error when sidecars inadvertently depend on each other": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"secondCar": "START",
					},
				},
				"secondCar": {
					DependsOn: map[string]string{
						"thirdCar": "START",
					},
				},
				"thirdCar": {
					DependsOn: map[string]string{
						"fourthCar": "START",
					},
				},
				"fourthCar": {
					DependsOn: map[string]string{
						"sidecar": "START",
					},
				},
			},
			wantErr:           circularDependencyErr,
			circDepContainers: []string{"sidecar", "secondCar", "thirdCar", "fourthCar"},
		},
		"error when container doesn't exist": {
			allSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					DependsOn: map[string]string{
						"something": "START",
					},
				},
			},
			wantErr: errInvalidContainer,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateNoCircularDependencies(convertSidecarOpts{
				sidecarConfig: tc.allSidecars,
				imageConfig:   &image,
				workloadName:  mockWorkloadName,
			})
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else if tc.wantErr == circularDependencyErr {
				require.Contains(t, gotErr.Error(), circularDependencyErr.Error())
				for _, container := range tc.circDepContainers {
					require.Contains(t, gotErr.Error(), container)
				}
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateImageDependsOn(t *testing.T) {
	mockWorkloadName := "frontend"
	testCases := map[string]struct {
		inImage    *manifest.Image
		inSidecars map[string]*manifest.SidecarConfig

		wantErr error
	}{
		"no image container dependencies": {
			inImage: &manifest.Image{},
			wantErr: nil,
		},
		"working image with container dependency": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "START",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: nil,
		},
		"error when image depends on itself": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"frontend": "START",
				},
			},
			wantErr: fmt.Errorf("container frontend cannot depend on itself"),
		},
		"error when image container dependency status is invalid": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "END",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errInvalidSidecarDependsOnStatus,
		},
		"error when set essential sidecar container has a status besides start": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "COMPLETE",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {
					Essential: aws.Bool(true),
				},
			},
			wantErr: errEssentialSidecarStatus,
		},
		"error when implied essential sidecar container has a status besides start": {
			inImage: &manifest.Image{
				DependsOn: map[string]string{
					"sidecar": "COMPLETE",
				},
			},
			inSidecars: map[string]*manifest.SidecarConfig{
				"sidecar": {},
			},
			wantErr: errEssentialSidecarStatus,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotErr := validateImageDependsOn(convertSidecarOpts{
				sidecarConfig: tc.inSidecars,
				imageConfig:   tc.inImage,
				workloadName:  mockWorkloadName,
			})
			if tc.wantErr == nil {
				require.NoError(t, gotErr)
			} else {
				require.EqualError(t, gotErr, tc.wantErr.Error())
			}
		})
	}
}

func Test_validatePubSubTopicName(t *testing.T) {
	testCases := map[string]struct {
		inName string

		wantErr error
	}{
		"valid topic name": {
			inName: "a-Perfectly_V4l1dString",
		},
		"error when no topic name": {
			inName:  "",
			wantErr: errMissingPublishTopicField,
		},
		"error when invalid topic name": {
			inName:  "OHNO~/`...,",
			wantErr: errInvalidPubSubTopicName,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validatePubSubName(tc.inName)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateWorkerName(t *testing.T) {
	testCases := map[string]struct {
		inName []string

		wantErr error
	}{
		"good case": {
			inName:  []string{"good-name"},
			wantErr: nil,
		},
		"empty name": {
			inName:  []string{""},
			wantErr: fmt.Errorf("worker name `` is invalid: %s", errInvalidSvcName),
		},
		"contains spaces": {
			inName:  []string{"a re@!!y b#d n&me"},
			wantErr: fmt.Errorf("worker name `a re@!!y b#d n&me` is invalid: %s", errSvcNameBadFormat),
		},
		"too long": {
			inName:  []string{"this-is-the-name-that-goes-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-until-it-is-too-long"},
			wantErr: fmt.Errorf("worker name `this-is-the-name-that-goes-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-and-on-until-it-is-too-long` is invalid: %s", errSvcNameTooLong),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateWorkerNames(tc.inName)

			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateTopicSubscription(t *testing.T) {
	validTopics := []string{"arn:aws:sns:us-east-1:123456789012:app-env-svc-name", "arn:aws:sns:us-east-1:123456789012:app-env-svc-name2"}
	testCases := map[string]struct {
		inTS manifest.TopicSubscription

		wantErr error
	}{
		"good case": {
			inTS: manifest.TopicSubscription{
				Name:    "name2",
				Service: "svc",
			},
			wantErr: nil,
		},
		"empty name": {
			inTS: manifest.TopicSubscription{
				Service: "svc",
			},
			wantErr: errMissingPublishTopicField,
		},
		"empty svc name": {
			inTS: manifest.TopicSubscription{
				Name: "theName",
			},
			wantErr: errInvalidSvcName,
		},
		"topic not in list of valid topics": {
			inTS: manifest.TopicSubscription{
				Name:    "badName",
				Service: "svc",
			},
			wantErr: errTopicSubscriptionNotAllowed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateTopicSubscription(tc.inTS, validTopics)

			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateTime(t *testing.T) {
	testCases := map[string]struct {
		inTime    time.Duration
		inFloor   float64
		inCeiling float64

		wantErr error
	}{
		"good case": {
			inTime:    500,
			inFloor:   0,
			inCeiling: 600,
			wantErr:   nil,
		},
		"bad time": {
			inTime:    601000000000,
			inFloor:   0,
			inCeiling: 600,
			wantErr:   errors.New("time must be between 0 and 600 seconds"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateTime(tc.inTime, tc.inFloor, tc.inCeiling)

			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateDeadLetter(t *testing.T) {
	testCases := map[string]struct {
		inDL *manifest.DeadLetterQueue

		wantErr error
	}{
		"good case": {
			inDL: &manifest.DeadLetterQueue{
				Tries: 35,
			},
			wantErr: nil,
		},
		"wrong number of tries": {
			inDL: &manifest.DeadLetterQueue{
				Tries: 9999,
			},
			wantErr: errDeadLetterQueueTries,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateDeadLetter(tc.inDL)

			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}
