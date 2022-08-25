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

type volumeConverter struct {
	otherSvcVols  map[string][]string
	copilotVols   map[string]*manifest.Volume
	topLevelVols  compose.Volumes
	tmpfsVolNames map[string]bool
}

func newVolumeConverter(topLevelVols compose.Volumes) volumeConverter {
	return volumeConverter{
		otherSvcVols:  map[string][]string{},
		copilotVols:   nil,
		topLevelVols:  topLevelVols,
		tmpfsVolNames: map[string]bool{},
	}
}

// convertVolumes converts a list of Compose volumes into the Copilot storage equivalent.
func (vc *volumeConverter) convertVolumes(volumes []compose.ServiceVolumeConfig, otherSvcs compose.Services) (*manifest.Storage, error) {
	for _, otherSvc := range otherSvcs {
		for _, vol := range otherSvc.Volumes {
			if vol.Type == "volume" {
				vc.otherSvcVols[vol.Source] = append(vc.otherSvcVols[vol.Source], otherSvc.Name)
			}
		}
	}

	var ephemeralBytes uint64 = 20 * units.GiB

	for idx, vol := range volumes {
		mountOpts := manifest.MountPointOpts{
			ContainerPath: aws.String(vol.Target),
			ReadOnly:      aws.Bool(vol.ReadOnly),
		}

		if vol.Type == "tmpfs" {
			ephemeralBytes += uint64(vol.Tmpfs.Size)

			name := fmt.Sprintf("tmpfs-%v", idx)

			if vc.copilotVols[name] != nil {
				return nil, fmt.Errorf("generated tmpfs volume name %s collides with an existing volume name", name)
			}

			vc.tmpfsVolNames[name] = true
			vc.copilotVols[name] = &manifest.Volume{
				EFS: manifest.EFSConfigOrBool{
					Enabled: aws.Bool(false),
				},
				MountPointOpts: mountOpts,
			}
			continue
		}

		if vol.Type != "volume" {
			// TODO (rclinard-amzn): Relax the "bind" restriction in Milestone 6
			return nil, fmt.Errorf("volume type %v is not supported yet", vol.Type)
		}

		name, err := vc.checkNamedVolume(vol)
		if err != nil {
			return nil, err
		}

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

	// default: 20 GiB
	if ephemeralGiB != 20 {
		storage.Ephemeral = aws.Int(int(ephemeralGiB))
	}

	if len(vc.copilotVols) != 0 {
		storage.Volumes = vc.copilotVols
	}

	return &storage, nil
}

func (vc *volumeConverter) checkNamedVolume(vol compose.ServiceVolumeConfig) (*string, error) {
	name := vol.Source
	if shared := vc.otherSvcVols[name]; shared != nil {
		return nil, fmt.Errorf("named volume %s is shared with %s [%s], this is not supported in Copilot",
			name, english.PluralWord(len(shared), "service", "services"), strings.Join(shared, ", "))
	}

	if vc.tmpfsVolNames[name] {
		return nil, fmt.Errorf("named volume %s collides with the generated name of a tmpfs mount", name)
	}

	if vc.copilotVols[name] != nil {
		return nil, fmt.Errorf("cannot mount named volume %s a second time at %s, it is already mounted at %s",
			name, vol.Target, *vc.copilotVols[name].ContainerPath)
	}

	if topLevel, ok := vc.topLevelVols[name]; ok {
		var err error
		var ignored IgnoredKeys

		switch {
		case topLevel.External.External:
			err = fmt.Errorf("named volume %s is marked as external, this is unsupported", name)
		case topLevel.Driver != "":
			err = fmt.Errorf("only the default driver is supported, but %s tries to use a different driver", name)
		case len(topLevel.Labels) != 0:
			ignored = append(ignored, fmt.Sprintf("volumes.%s.labels", name))
		case len(topLevel.DriverOpts) != 0:
			ignored = append(ignored, fmt.Sprintf("volumes.%s.driver_opts", name))
		}
		if err != nil {
			return nil, err
		}
	}

	return &name, nil
}
