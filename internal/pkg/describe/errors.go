// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/dustin/go-humanize/english"
)

// ErrManifestNotFoundInTemplate is returned when a deployed CFN template
// is missing manifest data.
type ErrManifestNotFoundInTemplate struct {
	app  string
	env  string
	name string
}

// Error implements the error interface.
func (err *ErrManifestNotFoundInTemplate) Error() string {
	return fmt.Sprintf("manifest metadata not found in template of stack %s-%s-%s", err.app, err.env, err.name)
}

type errLBWebSvcsOnCFWithoutAlias struct {
	services   []string
	aliasField string
}

// Error implements the error interface.
func (err *errLBWebSvcsOnCFWithoutAlias) Error() string {
	return fmt.Sprintf("%s %s must have %q specified when CloudFront is enabled", english.PluralWord(len(err.services), "service", "services"),
		english.WordSeries(template.QuoteSliceFunc(err.services), "and"), err.aliasField)
}

var errVPCIngressConnectionNotFound = errors.New("no vpc ingress connection found")
