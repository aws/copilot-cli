package dockercompose

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
)

// convertImageConfig takes in a Compose build config & image field and returns a Copilot image manifest.
func convertImageConfig(build *types.BuildConfig, imageLoc string) (manifest.Image, IgnoredKeys, error) {
	ignored, err := unsupportedBuildKeys(build)
	if err != nil {
		return manifest.Image{}, ignored, err
	}

	image := manifest.Image{
		DockerLabels: build.Labels,
	}

	// note: Compose allows both build & image to be specified, but image takes precedence.
	//       in Copilot, however, build & image are mutually exclusive.
	if imageLoc != "" {
		image.Location = &imageLoc
	} else {
		args, err := convertMappingWithEquals(build.Args)
		if err != nil {
			return manifest.Image{}, nil, fmt.Errorf("convert build args: %w", err)
		}

		image.Build = manifest.BuildArgsOrString{
			BuildArgs: manifest.DockerBuildArgs{
				Context:    &build.Context,
				Dockerfile: &build.Dockerfile,
				Args:       args,
				Target:     &build.Target,
				CacheFrom:  build.CacheFrom,
			},
		}
	}

	return image, ignored, nil
}

// convertMappingWithEquals checks for entries with missing values and generates an error if any are found.
func convertMappingWithEquals(inArgs types.MappingWithEquals) (map[string]string, error) {
	args := map[string]string{}
	var badArgs []string

	for k, v := range inArgs {
		if v == nil {
			badArgs = append(badArgs, k)
		} else {
			args[k] = *v
		}
	}

	if badArgs != nil {
		return nil, fmt.Errorf("some entries are missing values and require user input, "+
			"this is not supported in Copilot: %v", badArgs)
	}

	return args, nil
}

// unsupportedBuildKeys checks for unsupported keys in the given Compose build config.
func unsupportedBuildKeys(build *types.BuildConfig) (IgnoredKeys, error) {
	if build.SSH != nil || build.Secrets != nil {
		return nil, errors.New("`build.ssh` and `build.secrets` are not supported yet, see " +
			"https://github.com/aws/copilot-cli/issues/2090 for details")
	}

	if build.ExtraHosts != nil {
		return nil, errors.New("key `build.extra_hosts` is not supported yet, this might break your app")
	}

	if build.Network != "" {
		return nil, errors.New("key `build.network` is not supported yet, this might break your app")
	}

	var ignoredKeys []string

	if build.CacheTo != nil {
		ignoredKeys = append(ignoredKeys, "build.cache_to")
	}

	if build.NoCache {
		ignoredKeys = append(ignoredKeys, "build.no_cache")
	}

	if build.Pull {
		ignoredKeys = append(ignoredKeys, "build.pull")
	}

	if build.Isolation != "" {
		ignoredKeys = append(ignoredKeys, "build.isolation")
	}

	if build.Tags != nil {
		ignoredKeys = append(ignoredKeys, "build.tags")
	}

	if build.Extensions != nil {
		ignoredKeys = append(ignoredKeys, "build.extensions")
	}

	return ignoredKeys, nil
}
