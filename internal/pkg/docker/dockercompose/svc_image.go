// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockercompose

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	compose "github.com/compose-spec/compose-go/types"
	"github.com/dustin/go-humanize/english"
)

// convertImageConfig takes in a Compose build config & image field and returns a Copilot image manifest.
func convertImageConfig(build *compose.BuildConfig, labels map[string]string, imageLoc string) (manifest.Image, IgnoredKeys, error) {
	image := manifest.Image{
		DockerLabels: labels,
	}

	// Compose allows both build & image to be specified, but image takes precedence.
	// In Copilot, however, build and image are mutually exclusive.
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

	buildArgs := manifest.DockerBuildArgs{
		Context:    nilIfEmpty(build.Context),
		Dockerfile: nilIfEmpty(build.Dockerfile),
		Target:     nilIfEmpty(build.Target),
		CacheFrom:  build.CacheFrom,
	}

	args, err := convertMappingWithEquals(build.Args)
	if err != nil {
		return manifest.Image{}, nil, fmt.Errorf("convert build args: %w", err)
	}

	if len(args) != 0 {
		buildArgs.Args = args
	}

	image.Build = manifest.BuildArgsOrString{
		BuildArgs: buildArgs,
	}

	return image, ignored, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// convertMappingWithEquals checks for entries with missing values and generates an error if any are found.
func convertMappingWithEquals(inArgs compose.MappingWithEquals) (map[string]string, error) {
	args := map[string]string{}
	var badArgs []string

	for k, v := range inArgs {
		if v == nil {
			badArgs = append(badArgs, k)
			continue
		}
		args[k] = *v
	}

	if len(badArgs) != 0 {
		return nil, fmt.Errorf("%s '%v' %s missing %s and %s user input, this is unsupported in Copilot",
			english.PluralWord(len(badArgs), "entry", "entries"), badArgs,
			english.PluralWord(len(badArgs), "is", "are"),
			english.PluralWord(len(badArgs), "a value", "values"),
			english.PluralWord(len(badArgs), "requires", "require"))
	}

	return args, nil
}

// unsupportedBuildKeys checks for unsupported keys in the given Compose build config.
func unsupportedBuildKeys(build *compose.BuildConfig) (IgnoredKeys, error) {
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
		// catchall for any unrecognized key
		for ext := range build.Extensions {
			ignoredKeys = append(ignoredKeys, "build."+ext)
		}
	}

	if build.Labels != nil {
		ignoredKeys = append(ignoredKeys, "build.labels")
	}

	return ignoredKeys, nil
}
