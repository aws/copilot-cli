// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

var (
	errValueEmpty        = errors.New("value must not be empty")
	errValueTooLong      = errors.New("value must not exceed 255 characters")
	errValueBadFormat    = errors.New("value must start with a letter and contain only lower-case letters, numbers, and hyphens")
	errValueNotAString   = errors.New("value must be a string")
	errInvalidGitHubRepo = errors.New("value must be a valid GitHub repository, e.g. https://github.com/myCompany/myRepo")
	errPortInvalid       = errors.New("value must be in range 1-65535")
)

var githubRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

func validateProjectName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("project name %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("service name %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcPort(val interface{}) error {

	if err := basicPortValidation(val); err != nil {
		return fmt.Errorf("port %v is invalid: %w", val, err)
	}
	return nil
}

func validateSvcType(val interface{}) error {
	svcType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	for _, validType := range manifest.ServiceTypes {
		if svcType == validType {
			return nil
		}
	}
	var prettyTypes []string
	for _, validType := range manifest.ServiceTypes {
		prettyTypes = append(prettyTypes, fmt.Sprintf(`"%s"`, validType))
	}
	return fmt.Errorf("invalid service type %s: must be one of %s", svcType, strings.Join(prettyTypes, ", "))
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

func basicPortValidation(val interface{}) error {

	var err error

	switch val := val.(type) {
	case []byte:
		err = bytePortValidation(val)
	case string:
		err = stringPortValidation(val)
	case uint16:
		if val == 0 {
			err = errPortInvalid
		}
	default:
		err = errPortInvalid
	}
	return err
}

func bytePortValidation(val []byte) error {
	s := string(val)
	err := stringPortValidation(s)
	if err != nil {
		return err
	}
	return nil
}

func stringPortValidation(val string) error {
	port64, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return errPortInvalid
	}
	if port64 < 1 || port64 > 65535 {
		return errPortInvalid
	}
	return nil
}
