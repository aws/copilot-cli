// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

var (
	errValueEmpty           = errors.New("value must not be empty")
	errValueTooLong         = errors.New("value must not exceed 255 characters")
	errValueBadFormat       = errors.New("value must start with a letter and contain only lower-case letters, numbers, and hyphens")
	errEnvVarValueBadFormat = errors.New("value must start with a letter and contain only upper-case letters, and underscores")
	errValueNotAString      = errors.New("value must be a string")
	errInvalidGitHubRepo    = errors.New("value must be a valid GitHub repository, e.g. https://github.com/myCompany/myRepo")
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

func noValidation(val interface{}) error {
	return nil
}

func validateEnvVarName(val interface{}) error {
	if err := envVarValidation(val); err != nil {
		return fmt.Errorf("env var name %v is invalid: %w", val, err)
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

func envVarValidation(val interface{}) error {
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
	if !isCorrectEnvVarFormat(s) {
		return errEnvVarValueBadFormat
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

func isCorrectEnvVarFormat(s string) bool {
	valid, err := regexp.MatchString(`^[A-Z][A-Z_]+$`, s)
	if err != nil {
		return false // bubble up error?
	}
	return valid
}
