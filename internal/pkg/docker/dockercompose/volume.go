// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/docker/go-units"
	"github.com/dustin/go-humanize/english"
	"strings"
)

// The default size that Copilot reserves for a service's ephemeral on-disk storage.
const defaultEphemeralGiB = 20
const defaultEphemeralBytes uint64 = defaultEphemeralGiB * units.GiB

type volumeConverter struct {
	otherSvcVols  map[string][]string
	copilotVols   map[string]*manifest.Volume
	topLevelVols  compose.Volumes
	tmpfsVolNames map[string]bool
}

func newVolumeConverter(topLevelVols compose.Volumes) volumeConverter {
	return volumeConverter{
		otherSvcVols:  map[string][]string{},
		copilotVols:   map[string]*manifest.Volume{},
		topLevelVols:  topLevelVols,
		tmpfsVolNames: map[string]bool{},
	}
}

// convertVolumes converts a list of Compose volumes into the Copilot storage equivalent.
func (vc *volumeConverter) convertVolumes(volumes []compose.ServiceVolumeConfig, otherSvcs compose.Services) (*manifest.Storage, IgnoredKeys, error) {
	for _, otherSvc := range otherSvcs {
		for _, vol := range otherSvc.Volumes {
			if vol.Type == "volume" {
				vc.otherSvcVols[vol.Source] = append(vc.otherSvcVols[vol.Source], otherSvc.Name)
			}
		}
	}

	var ephemeralBytes = defaultEphemeralBytes
	var ignored IgnoredKeys

	for idx, vol := range volumes {
		if vol.Target == "" {
			return nil, nil, fmt.Errorf("volume mounted from %q (type %q) is missing a target mount point", vol.Source, vol.Type)
		}

		mountOpts := manifest.MountPointOpts{
			ContainerPath: aws.String(vol.Target),
			ReadOnly:      aws.Bool(vol.ReadOnly),
		}

		if vol.Type == "tmpfs" {
			ephemeralBytes += uint64(vol.Tmpfs.Size)

			name := fmt.Sprintf("tmpfs-%v", idx)

			if vc.copilotVols[name] != nil {
				return nil, nil, fmt.Errorf("generated tmpfs volume name %s collides with an existing volume name", name)
			}

			vc.tmpfsVolNames[name] = true
			vc.copilotVols[name] = &manifest.Volume{
				// efs is off by default
				MountPointOpts: mountOpts,
			}
			continue
		}

		if vol.Type != "volume" {
			// TODO (rclinard-amzn): Relax the "bind" restriction in Milestone 6
			return nil, nil, fmt.Errorf("volume type %q is not supported yet", vol.Type)
		}

		name, ign, err := vc.checkNamedVolume(vol)
		if err != nil {
			return nil, nil, err
		}
		ignored = append(ignored, ign...)

		vc.copilotVols[*name] = &manifest.Volume{
			EFS: manifest.EFSConfigOrBool{
				Enabled: aws.Bool(true),
			},
			MountPointOpts: mountOpts,
		}
	}

	// math trick to round up to the next highest GiB if it's not an even size in GiB
	ephemeralGiB := (ephemeralBytes + units.GiB - 1) / units.GiB
	var storage manifest.Storage

	if ephemeralGiB != defaultEphemeralGiB {
		storage.Ephemeral = aws.Int(int(ephemeralGiB))
	}

	if len(vc.copilotVols) != 0 {
		storage.Volumes = vc.copilotVols
	}

	return &storage, ignored, nil
}

func (vc *volumeConverter) checkNamedVolume(vol compose.ServiceVolumeConfig) (*string, IgnoredKeys, error) {
	name := vol.Source
	if shared := vc.otherSvcVols[name]; shared != nil {
		return nil, nil, fmt.Errorf("named volume %s is shared with %s [%s], this is not supported in Copilot",
			name, english.PluralWord(len(shared), "service", "services"), strings.Join(shared, ", "))
	}

	if vc.tmpfsVolNames[name] {
		return nil, nil, fmt.Errorf("named volume %s collides with the generated name of a tmpfs mount", name)
	}

	if vc.copilotVols[name] != nil {
		return nil, nil, fmt.Errorf("cannot mount named volume %s a second time at %s, it is already mounted at %s",
			name, vol.Target, *vc.copilotVols[name].ContainerPath)
	}

	var ignored IgnoredKeys

	if topLevel, ok := vc.topLevelVols[name]; ok {
		var err error

		switch {
		case topLevel.External.External:
			err = fmt.Errorf("named volume %s is marked as external, this is unsupported", name)
		case topLevel.Driver != "":
			err = fmt.Errorf("only the default driver is supported, but the volume %s tries to use a different driver", name)
		case len(topLevel.DriverOpts) != 0:
			ignored = append(ignored, fmt.Sprintf("volumes.%s.driver_opts", name))
			fallthrough
		case len(topLevel.Labels) != 0:
			ignored = append(ignored, fmt.Sprintf("volumes.%s.labels", name))
		}
		if err != nil {
			return nil, nil, err
		}
	}

	return &name, ignored, nil
}
