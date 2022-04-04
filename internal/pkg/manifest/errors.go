// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"strconv"

	"github.com/dustin/go-humanize/english"
)

// ErrInvalidWorkloadType occurs when a user requested a manifest template type that doesn't exist.
type ErrInvalidWorkloadType struct {
	Type string
}

func (e *ErrInvalidWorkloadType) Error() string {
	return fmt.Sprintf("invalid manifest type: %s", e.Type)
}

// ErrInvalidPipelineManifestVersion occurs when the pipeline.yml file
// contains invalid schema version during unmarshalling.
type ErrInvalidPipelineManifestVersion struct {
	invalidVersion PipelineSchemaMajorVersion
}

func (e *ErrInvalidPipelineManifestVersion) Error() string {
	return fmt.Sprintf("manifest.yml contains invalid schema version: %d", e.invalidVersion)
}

// Is compares the 2 errors. Only returns true if the errors are of the same
// type and contain the same information.
func (e *ErrInvalidPipelineManifestVersion) Is(target error) bool {
	t, ok := target.(*ErrInvalidPipelineManifestVersion)
	return ok && t.invalidVersion == e.invalidVersion
}

// ErrUnknownProvider occurs CreateProvider() is called with configurations
// that do not map to any supported provider.
type ErrUnknownProvider struct {
	unknownProviderProperties interface{}
}

func (e *ErrUnknownProvider) Error() string {
	return fmt.Sprintf("no provider found for properties: %v",
		e.unknownProviderProperties)
}

// Is compares the 2 errors. Returns true if the errors are of the same
// type
func (e *ErrUnknownProvider) Is(target error) bool {
	_, ok := target.(*ErrUnknownProvider)
	return ok
}

type errFieldMustBeSpecified struct {
	missingField      string
	conditionalFields []string
}

func (e *errFieldMustBeSpecified) Error() string {
	errMsg := fmt.Sprintf(`"%s" must be specified`, e.missingField)
	if len(e.conditionalFields) == 0 {
		return errMsg
	}
	return fmt.Sprintf(`%s if "%s" %s specified`, errMsg, english.WordSeries(e.conditionalFields, "or"),
		english.PluralWord(len(e.conditionalFields), "is", "are"))
}

type errFieldMutualExclusive struct {
	firstField  string
	secondField string
	mustExist   bool
}

func (e *errFieldMutualExclusive) Error() string {
	if e.mustExist {
		return fmt.Sprintf(`must specify one of "%s" and "%s"`, e.firstField, e.secondField)
	}
	return fmt.Sprintf(`must specify one, not both, of "%s" and "%s"`, e.firstField, e.secondField)
}

type errMinGreaterThanMax struct {
	min int
	max int
}

func (e *errMinGreaterThanMax) Error() string {
	return fmt.Sprintf("min value %d cannot be greater than max value %d", e.min, e.max)
}

type errAtLeastOneFieldMustBeSpecified struct {
	missingFields    []string
	conditionalField string
}

func (e *errAtLeastOneFieldMustBeSpecified) Error() string {
	quotedFields := make([]string, len(e.missingFields))
	for i, f := range e.missingFields {
		quotedFields[i] = strconv.Quote(f)
	}
	errMsg := fmt.Sprintf("must specify at least one of %s", english.WordSeries(quotedFields, "or"))
	if e.conditionalField != "" {
		errMsg = fmt.Sprintf(`%s if "%s" is specified`, errMsg, e.conditionalField)
	}
	return errMsg
}
