// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/afero"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

const (
	// IaC options for overrides.
	cdkIaCTool = "cdk"

	// IaC toolkit configuration.
	typescriptCDKLang = "typescript"
)

var validIaCTools = []string{
	cdkIaCTool,
}

var validCDKLangs = []string{
	typescriptCDKLang,
}

type stringWriteCloser interface {
	fmt.Stringer
	io.WriteCloser
}

type closableStringBuilder struct {
	*strings.Builder
}

// Close implements the io.Closer interface for a strings.Builder and is a no-op.
func (sb *closableStringBuilder) Close() error {
	return nil
}

// overrideVars represent common flags for all "[noun] override" commands.
type overrideVars struct {
	name    string
	appName string
	iacTool string

	// CDK override engine flags.
	cdkLang string

	// We prompt for resources if the user does not opt-in to skipping.
	skipResources bool
	resources     []template.CFNResource
}

// overrideOpts represents the command for all "[noun] override" commands.
type overrideOpts struct {
	overrideVars

	// Interfaces to interact with dependencies.
	fs         afero.Fs
	cfgStore   store
	prompt     prompter
	cfnPrompt  cfnSelector
	packageCmd func(w stringWriteCloser) (executor, error)
}

// Validate returns an error for any invalid optional flags.
func (o *overrideOpts) Validate() error {
	if err := o.validateAppName(); err != nil {
		return err
	}
	return o.validateCDKLang()
}

func (o *overrideOpts) validateAppName() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	_, err := o.cfgStore.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %q configuration: %v", o.appName, err)
	}
	return nil
}

func (o *overrideOpts) validateCDKLang() error {
	for _, valid := range validCDKLangs {
		if o.cdkLang == valid {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid CDK language: must be one of: %s",
		o.cdkLang,
		strings.Join(applyAll(validCDKLangs, strconv.Quote), ", "))
}
