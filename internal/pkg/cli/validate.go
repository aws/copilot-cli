// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

var (
	errValueEmpty        = errors.New("value must not be empty")
	errValueTooLong      = errors.New("value must not exceed 255 characters")
	errValueBadFormat    = errors.New("value must start with a letter and contain only lower-case letters, numbers, and hyphens")
	errValueNotAString   = errors.New("value must be a string")
	errInvalidGitHubRepo = errors.New("value must be a valid GitHub repository, e.g. https://github.com/myCompany/myRepo")
	errPortInvalid       = errors.New("value must be in range 1-65535")
	errS3ValueBadSize    = errors.New("value must be between 3 and 63 characters in length")
	errS3ValueBadFormat  = errors.New("value must contain only alphanumeric characters and .-")
	errDDBValueBadSize   = errors.New("value must be between 3 and 255 characters in length")
	errDDBValueBadFormat = errors.New("value must contain only alphanumeric characters and ._-")
)

var fmtErrInvalidStorageType = "invalid storage type %s: must be one of %s"

var githubRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

func validateAppName(val interface{}) error {
	if err := basicNameValidation(val); err != nil {
		return fmt.Errorf("application name %v is invalid: %w", val, err)
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
func prettify(inputStrings []string) string {
	var prettyTypes []string
	for _, validType := range inputStrings {
		prettyTypes = append(prettyTypes, fmt.Sprintf(`"%s"`, validType))
	}
	return strings.Join(prettyTypes, ", ")
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

	return fmt.Errorf("invalid service type %s: must be one of %s", svcType, prettify(manifest.ServiceTypes))
}

func validateStorageType(val interface{}) error {
	storageType, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	for _, validType := range storageTypes {
		if storageType == validType {
			return nil
		}
	}
	return fmt.Errorf(fmtErrInvalidStorageType, storageType, prettify(storageTypes))
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

// s3 bucket names: 'a-zA-Z0-9.-'
func s3BucketNameValidation(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < 3 || len(s) > 63 {
		return errS3ValueBadSize
	}
	for _, r := range s {
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '.' || r == '-') {
			return errS3ValueBadFormat
		}
	}
	return nil
}

// Dynameo table names: 'a-zA-Z0-9.-_'
func dynamoTableNameValidation(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < 3 || len(s) > 255 {
		return errDDBValueBadSize
	}
	for _, r := range s {
		if !(unicode.IsDigit(r) || unicode.IsLetter(r) || r == '.' || r == '-' || r == '_') {
			return errDDBValueBadFormat
		}
	}
	return nil
}
