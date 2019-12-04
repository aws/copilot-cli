// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/spf13/afero"
)

var (
	errValueEmpty        = errors.New("value must not be empty")
	errValueTooLong      = errors.New("value must not exceed 255 characters")
	errValueBadFormat    = errors.New("value must start with a letter and contain only lower-case letters, numbers, and hyphens")
	errValueNotAString   = errors.New("value must be a string")
	errInvalidGitHubRepo = errors.New("value must be a valid GitHub repository, e.g. https://github.com/myCompany/myRepo")
)

var githubRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

func validateProjectName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("project name %v is invalid: %w", val, err)
	}
	return nil
}

func validateApplicationName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("application name %v is invalid: %w", val, err)
	}
	return nil
}

func validateApplicationType(val interface{}) error {
	appType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	for _, validType := range manifest.AppTypes {
		if appType == validType {
			return nil
		}
	}
	var prettyTypes []string
	for _, validType := range manifest.AppTypes {
		prettyTypes = append(prettyTypes, fmt.Sprintf(`"%s"`, validType))
	}
	return fmt.Errorf("invalid app type %s: must be one of %s", appType, strings.Join(prettyTypes, ", "))
}

func validateEnvironmentName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("environment name %v is invalid: %w", val, err)
	}
	return nil
}

func basicNameValidation(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if s == "" {
		return errValueEmpty
	}
	if len(s) > 255 {
		return errValueTooLong
	}
	if !isCorrectFormat(s) {
		return errValueBadFormat
	}

	return nil
}

func isCorrectFormat(s string) bool {
	valid, err := regexp.MatchString(`^[a-z][a-z0-9\-]+$`, s)
	if err != nil {
		return false // bubble up error?
	}
	return valid
}

func validateGitHubRepo(val interface{}) error {
	repo, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if !githubRepoExp.MatchString(repo) {
		return fmt.Errorf("GitHub repository name %v is invalid. %w", val, errInvalidGitHubRepo)
	}
	return nil
}

// validateDockerfiles returns the list of Dockerfiles within the current
// working directory and a sub-directory level below. If an error occurs while
// reading directories, or no Dockerfiles found returns the error.
func validateDockerfiles(fs afero.Fs) ([]string, error) {
	wdFiles, err := afero.ReadDir(fs, ".")
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var directories []string

	for _, wdFile := range wdFiles {
		// Add current directory if a Dockerfile exists, otherwise continue.
		if !wdFile.IsDir() {
			if wdFile.Name() == "Dockerfile" {
				directories = append(directories, filepath.Dir(wdFile.Name()))
			}
			continue
		}

		// Add sub-directories containing a Dockerfile one level below current directory.
		subFiles, err := afero.ReadDir(fs, wdFile.Name())
		if err != nil {
			return nil, fmt.Errorf("read directory: %w", err)
		}
		for _, f := range subFiles {
			// NOTE: ignore directories in sub-directories.
			if f.IsDir() {
				continue
			}

			if f.Name() == "Dockerfile" {
				directories = append(directories, wdFile.Name())
			}
		}
	}
	if len(directories) == 0 {
		return nil, fmt.Errorf("no Dockerfiles found within the current working directory or a sub-directory level below")
	}
	sort.Strings(directories)
	var dockerfiles []string
	for _, dir := range directories {
		file := dir + "/Dockerfile"
		dockerfiles = append(dockerfiles, file)
	}
	return dockerfiles, nil
}
