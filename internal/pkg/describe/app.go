// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// App contains serialized parameters for an application.
type App struct {
	Name                string                   `json:"name"`
	Version             string                   `json:"version"`
	URI                 string                   `json:"uri"`
	PermissionsBoundary string                   `json:"permissionsBoundary"`
	Envs                []*config.Environment    `json:"environments"`
	Services            []*config.Workload       `json:"services"`
	Jobs                []*config.Workload       `json:"jobs"`
	Pipelines           []*codepipeline.Pipeline `json:"pipelines"`
	WkldDeployedtoEnvs  map[string][]string      `json:"-"`
}

// JSONString returns the stringified App struct with json format.
func (a *App) JSONString() (string, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("marshal application description: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified App struct with human readable format.
func (a *App) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprint(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", a.Name)
	if a.URI == "" {
		a.URI = "N/A"
	}
	if a.PermissionsBoundary == "" {
		a.PermissionsBoundary = "N/A"
	}
	fmt.Fprintf(writer, "  %s\t%s\n", "Version", a.Version)
	fmt.Fprintf(writer, "  %s\t%s\n", "URI", a.URI)
	fmt.Fprintf(writer, "  %s\t%s\n", "Permissions Boundary", a.PermissionsBoundary)
	fmt.Fprint(writer, color.Bold.Sprint("\nEnvironments\n\n"))
	writer.Flush()
	headers := []string{"Name", "AccountID", "Region"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, env := range a.Envs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", env.Name, env.AccountID, env.Region)
	}
	fmt.Fprint(writer, color.Bold.Sprint("\nWorkloads\n\n"))
	writer.Flush()
	headers = []string{"Name", "Type", "Environments"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, svc := range a.Services {
		envs := "-"
		if len(a.WkldDeployedtoEnvs[svc.Name]) > 0 {
			envs = strings.Join(a.WkldDeployedtoEnvs[svc.Name], ", ")
		}
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", svc.Name, svc.Type, envs)
	}
	for _, job := range a.Jobs {
		envs := "-"
		if len(a.WkldDeployedtoEnvs[job.Name]) > 0 {
			envs = strings.Join(a.WkldDeployedtoEnvs[job.Name], ", ")
		}
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", job.Name, job.Type, envs)
	}
	writer.Flush()
	fmt.Fprint(writer, color.Bold.Sprint("\nPipelines\n\n"))
	writer.Flush()
	headers = []string{"Name"}
	fmt.Fprintf(writer, "  %s\n", strings.Join(headers, "\t"))
	fmt.Fprintf(writer, "  %s\n", strings.Join(underline(headers), "\t"))
	for _, pipeline := range a.Pipelines {
		fmt.Fprintf(writer, "  %s\n", pipeline.Name)
	}
	writer.Flush()
	return b.String()
}

// AppDescriber retrieves information about an application.
type AppDescriber struct {
	app               string
	stackDescriber    stackDescriber
	stackSetDescriber stackDescriber
}

// NewAppDescriber instantiates an application describer.
func NewAppDescriber(appName string) (*AppDescriber, error) {
	sess, err := sessions.ImmutableProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("assume default role for app %s: %w", appName, err)
	}
	return &AppDescriber{
		app:               appName,
		stackDescriber:    stack.NewStackDescriber(cfnstack.NameForAppStack(appName), sess),
		stackSetDescriber: stack.NewStackDescriber(cfnstack.NameForAppStackSet(appName), sess),
	}, nil
}

// Version returns the app CloudFormation template version associated with
// the application by reading the Metadata.Version field from the template.
// Specifically it will get both app CFN stack template version and app StackSet template version,
// and return the minimum as the current app version.
//
// If the Version field does not exist, then it's a legacy template and it returns an deploy.LegacyAppTemplate and nil error.
func (d *AppDescriber) Version() (string, error) {
	type metadata struct {
		TemplateVersion string `yaml:"TemplateVersion"`
	}
	stackMetadata, stackSetMetadata := metadata{}, metadata{}

	appStackMetadata, err := d.stackDescriber.StackMetadata()
	if err != nil {
		return "", err
	}
	if err := yaml.Unmarshal([]byte(appStackMetadata), &stackMetadata); err != nil {
		return "", fmt.Errorf("unmarshal Metadata property from app %s stack: %w", d.app, err)
	}
	appStackVersion := stackMetadata.TemplateVersion
	if appStackVersion == "" {
		appStackVersion = version.LegacyAppTemplate
	}

	appStackSetMetadata, err := d.stackSetDescriber.StackSetMetadata()
	if err != nil {
		return "", err
	}
	if err := yaml.Unmarshal([]byte(appStackSetMetadata), &stackSetMetadata); err != nil {
		return "", fmt.Errorf("unmarshal Metadata property for app %s stack set: %w", d.app, err)
	}
	appStackSetVersion := stackSetMetadata.TemplateVersion
	if appStackSetVersion == "" {
		appStackSetVersion = version.LegacyAppTemplate
	}

	minVersion := appStackVersion
	if semver.Compare(appStackVersion, appStackSetVersion) > 0 {
		minVersion = appStackSetVersion
	}
	return minVersion, nil
}
