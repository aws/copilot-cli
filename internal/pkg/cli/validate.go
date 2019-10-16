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
	errValueEmpty      = errors.New("value must not be empty")
	errValueTooLong    = errors.New("value must not exceed 255 characters")
	errValueBadFormat  = errors.New("value must be start with letter and container only letters, numbers, and hyphens")
	errValueNotAString = errors.New("value must be a string")
)

func validateProjectName(val interface{}) error {
	return basicNameValidation(val)
}

func validateApplicationName(val interface{}) error {
	return basicNameValidation(val)
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
	return basicNameValidation(val)
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
	valid, err := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9\-]+$`, s)
	if err != nil {
		return false // bubble up error?
	}
	return valid
}
