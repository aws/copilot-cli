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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
)

var (
	errValueEmpty                         = errors.New("value must not be empty")
	errValueTooLong                       = errors.New("value must not exceed 255 characters")
	errValueBadFormat                     = errors.New("value must start with a letter and contain only lower-case letters, numbers, and hyphens")
	errValueNotAString                    = errors.New("value must be a string")
	errValueNotAStringSlice               = errors.New("value must be a string slice")
	errInvalidGitHubRepo                  = errors.New("value must be a valid GitHub repository, e.g. https://github.com/myCompany/myRepo")
	errPortInvalid                        = errors.New("value must be in range 1-65535")
	errS3ValueBadSize                     = errors.New("value must be between 3 and 63 characters in length")
	errS3ValueBadFormat                   = errors.New("value must not contain consecutive periods or dashes, or be formatted as IP address")
	errS3ValueTrailingDash                = errors.New("value must not have trailing -")
	errValueBadFormatWithPeriod           = errors.New("value must contain only lowercase alphanumeric characters and .-")
	errDDBValueBadSize                    = errors.New("value must be between 3 and 255 characters in length")
	errValueBadFormatWithPeriodUnderscore = errors.New("value must contain only alphanumeric characters and ._-")
	errDDBAttributeBadFormat              = errors.New("value must be of the form <name>:<T> where T is one of S, N, or B")
	errLsiAttributeNotPresent             = errors.New("lsi must be present in list of attributes")
	errTooManyLsiKeys                     = errors.New("number of specified LSI sort keys must be 5 or less")
)

var fmtErrInvalidStorageType = "invalid storage type %s: must be one of %s"

var githubRepoExp = regexp.MustCompile(`(https:\/\/github\.com\/|)(?P<owner>.+)\/(?P<repo>.+)`)

// matches alphanumeric, ._-, from 3 to 255 characters long
// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.NamingRulesDataTypes.html
var ddbRegExp = regexp.MustCompile(`^[a-zA-Z0-9\-\.\_]+$`)

// s3 validation expressions.
// s3RegExp matches alphanumeric, .- from 3 to 63 characters long.
// s3DashesRegExp matches consecutive dashes or periods
// s3TrailingDashRegExp matches a trailing dash
// ipAddressRegExp checks for a bucket in the format of an IP address.
// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-s3-bucket-naming-requirements.html
var (
	s3RegExp = regexp.MustCompile("" +
		`^` + // start of line
		`[a-z0-9\.\-]{3,63}` + // main match: lowercase alphanumerics, ., - from 3-63 characters
		`$`, // end of line
	)
	s3DashesRegExp = regexp.MustCompile(
		`[\.\-]{2,}`, // check for consecutive periods or dashes
	)
	s3TrailingDashRegExp = regexp.MustCompile(
		`-$`, // check for trailing dash
	)
	ipAddressRegexp = regexp.MustCompile(
		`^(?:\d{1,3}\.){3}\d{1,3}$`, // match any 1-3 digits in xxx.xxx.xxx.xxx format.
	)
)

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

// s3 bucket names: 'a-z0-9.-'
func s3BucketNameValidation(val interface{}) error {
	const minS3Length = 3
	const maxS3Length = 63
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minS3Length || len(s) > maxS3Length {
		return errS3ValueBadSize
	}

	// Check for bad punctuation (no consecutive dashes or dots)
	formatMatch := s3DashesRegExp.FindStringSubmatch(s)
	if len(formatMatch) != 0 {
		return errS3ValueBadFormat
	}

	// check for correct character set
	nameMatch := s3RegExp.FindStringSubmatch(s)
	if len(nameMatch) == 0 {
		return errValueBadFormatWithPeriod
	}

	dashMatch := s3TrailingDashRegExp.FindStringSubmatch(s)
	if len(dashMatch) != 0 {
		return errS3ValueTrailingDash
	}

	ipMatch := ipAddressRegexp.FindStringSubmatch(s)
	if len(ipMatch) != 0 {
		return errS3ValueBadFormat
	}

	return nil
}

// Dynameo table names: 'a-zA-Z0-9.-_'
func dynamoTableNameValidation(val interface{}) error {
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/HowItWorks.NamingRulesDataTypes.html
	const minDDBTableNameLength = 3
	const maxDDBTableNameLength = 255

	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if len(s) < minDDBTableNameLength || len(s) > maxDDBTableNameLength {
		return errDDBValueBadSize
	}
	m := ddbRegExp.FindStringSubmatch(s)
	if len(m) == 0 {
		return errValueBadFormatWithPeriodUnderscore
	}
	return nil
}

func validateKey(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	attr, err := getAttrFromKey(s)
	if err != nil {
		return errDDBAttributeBadFormat
	}
	err = dynamoTableNameValidation(attr.name)
	if err != nil {
		return errValueBadFormatWithPeriodUnderscore
	}
	err = validateDynamoDataType(attr.dataType)
	if err != nil {
		return err
	}
	return nil
}

func validateDynamoDataType(val interface{}) error {
	s, ok := val.(string)
	if !ok {
		return errValueNotAString
	}
	if !strings.Contains("SNB", strings.ToUpper(s)) {
		return errDDBAttributeBadFormat
	}
	return nil
}

func validateLSIs(val interface{}) error {
	s, ok := val.([]string)
	if !ok {
		return errValueNotAStringSlice
	}
	if len(s) > 5 {
		return errTooManyLsiKeys
	}
	for _, att := range s {
		err := validateKey(att)
		if err != nil {
			return err
		}
	}
	return nil
}

func prettify(inputStrings []string) string {
	prettyTypes := template.QuoteSliceFunc(inputStrings)
	return strings.Join(prettyTypes, ", ")
}
