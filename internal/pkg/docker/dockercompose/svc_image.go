package dockercompose

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/compose-spec/compose-go/types"
)

// convertImageConfig takes in a Compose build config & image field and returns a Copilot image manifest.
func convertImageConfig(build *types.BuildConfig, labels map[string]string, imageLoc string) (manifest.Image, IgnoredKeys, error) {
	image := manifest.Image{
		DockerLabels: labels,
	}

	// note: Compose allows both build & image to be specified, but image takes precedence.
	//       in Copilot, however, build & image are mutually exclusive.
	if imageLoc != "" {
		image.Location = aws.String(imageLoc)
		return image, nil, nil
	} else if build == nil {
		return manifest.Image{}, nil, errors.New("missing one of `build` or `image`")
	}

	ignored, err := unsupportedBuildKeys(build)
	if err != nil {
		return manifest.Image{}, ignored, err
	}

	args, err := convertMappingWithEquals(build.Args)
	if err != nil {
		return manifest.Image{}, nil, fmt.Errorf("convert build args: %w", err)
	}

	buildArgs := manifest.DockerBuildArgs{
		Context:    nilIfEmpty(build.Context),
		Dockerfile: nilIfEmpty(build.Dockerfile),
		Args:       args,
		Target:     nilIfEmpty(build.Target),
		CacheFrom:  build.CacheFrom,
	}

	image.Build = manifest.BuildArgsOrString{
		BuildArgs: buildArgs,
	}

	return image, ignored, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	} else {
		return &s
	}
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

	if len(args) == 0 {
		return nil, nil
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

	if build.Labels != nil {
		ignoredKeys = append(ignoredKeys, "build.labels")
	}

	if len(ignoredKeys) == 0 {
		return nil, nil
	}

	return ignoredKeys, nil
}
