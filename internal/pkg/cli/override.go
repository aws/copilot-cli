// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/override"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/spf13/afero"
)

const (
	// IaC options for overrides.
	cdkIaCTool = "cdk"
	yamlPatch  = "yamlpatch"

	// IaC toolkit configuration.
	typescriptCDKLang = "typescript"
)

var validIaCTools = []string{
	cdkIaCTool,
	yamlPatch,
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
	requiresEnv   bool
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
	spinner    progress
	dir        func() string
	packageCmd func(w stringWriteCloser) (executor, error)
}

// Validate returns an error for any invalid optional flags.
func (o *overrideOpts) Validate() error {
	if err := o.validateAppName(); err != nil {
		return err
	}
	return o.validateCDKLang()
}

// Ask prompts for and validates any required flags.
func (o *overrideOpts) Ask() error {
	if err := o.validateOrAskIaCTool(); err != nil {
		return err
	}
	return o.askResourcesToOverride()
}

// Execute writes IaC override files to the local workspace.
// This method assumes that the IaC tool chosen by the user is valid.
func (o *overrideOpts) Execute() error {
	dir := o.dir()
	switch o.iacTool {
	case cdkIaCTool:
		if err := override.ScaffoldWithCDK(o.fs, dir, o.resources, o.requiresEnv); err != nil {
			return fmt.Errorf("scaffold CDK application under %q: %v", dir, err)
		}
		log.Successf("Created a new CDK application at %q to override resources\n", displayPath(dir))
	case yamlPatch:
		if err := override.ScaffoldWithPatch(o.fs, dir); err != nil {
			return fmt.Errorf("scaffold CFN YAML patches under %q: %v", dir, err)
		}
		log.Successf("Created a YAML patch file %q to override resources\n", filepath.Join(displayPath(dir), override.YAMLPatchFile))
	}
	return nil
}

// RecommendActions prints optional follow-up actions.
func (o *overrideOpts) RecommendActions() error {
	readmePath := filepath.Join(o.dir(), "README.md")
	logRecommendedActions([]string{
		fmt.Sprintf("Please follow the instructions in %q", displayPath(readmePath)),
	})
	return nil
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

func (o *overrideOpts) validateOrAskIaCTool() error {
	if o.iacTool == "" {
		return o.askIaCTool()
	}
	return o.validateIaCTool()
}

func (o *overrideOpts) askIaCTool() error {
	msg := fmt.Sprintf("Which Infrastructure as Code tool would you like to use to override %q?", o.name)
	help := `The AWS Cloud Development Kit (CDK) lets you override templates using
the expressive power of programming languages.
This option is recommended for users that need to override several resources.
To learn more about the CDK: https://aws.github.io/copilot-cli/docs/developing/overrides/cdk/

CloudFormation YAML patches is recommended for users that need to override
a handful resources or do not want to depend on any other tool.
To learn more about CFN yaml patches: https://aws.github.io/copilot-cli/docs/developing/overrides/yamlpatch/`
	tool, err := o.prompt.SelectOne(msg, help, validIaCTools, prompt.WithFinalMessage("IaC tool:"))
	if err != nil {
		return fmt.Errorf("select IaC tool: %v", err)
	}
	o.iacTool = tool
	return nil
}

func (o *overrideOpts) validateIaCTool() error {
	for _, valid := range validIaCTools {
		if o.iacTool == valid {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid IaC tool: must be one of: %s",
		o.iacTool,
		strings.Join(applyAll(validIaCTools, strconv.Quote), ", "))
}

func (o *overrideOpts) askResourcesToOverride() error {
	if o.skipResources || o.iacTool == yamlPatch {
		return nil
	}

	o.spinner.Start("Generating CloudFormation template for resource selection")
	buf := &closableStringBuilder{
		Builder: new(strings.Builder),
	}
	pkgCmd, err := o.packageCmd(buf)
	if err != nil {
		o.spinner.Stop("")
		return err
	}
	if err := pkgCmd.Execute(); err != nil {
		o.spinner.Stop("")
		return fmt.Errorf("generate CloudFormation template for %q: %v", o.name, err)
	}
	o.spinner.Stop("")
	msg := fmt.Sprintf("Which resources in %q would you like to override?", o.name)
	resources, err := o.cfnPrompt.Resources(msg, "Resources:", "", buf.String())
	if err != nil {
		return err
	}
	o.resources = resources
	return nil
}
