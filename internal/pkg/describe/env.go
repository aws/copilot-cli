// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	sess "github.com/aws/aws-sdk-go/aws/session"
)

type EnvDescription struct {
	Environment  *config.Environment `json:"environment"`
	Applications []*config.Service   `json:"applications"`
	Tags         map[string]string   `json:"tags,omitempty"`
}

// EnvDescriber retrieves information about an environment.
type EnvDescriber struct {
	env  *config.Environment
	apps []*config.Service

	store        storeSvc
	sessProvider *sess.Session
}

// NewEnvDescriber instantiates an environment describer.
func NewEnvDescriber(projectName string, envName string) (*EnvDescriber, error) {
	svc, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to store: %w", err)
	}
	env, err := svc.GetEnvironment(projectName, envName)
	if err != nil {
		return nil, err
	}
	apps, err := svc.ListServices(projectName)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, fmt.Errorf("assuming role for environment %s: %w", env.ManagerRoleARN, err)
	}
	return &EnvDescriber{
		env:          env,
		store:        svc,
		apps:         apps,
		sessProvider: sess,
	}, nil
}

// Describe returns info about a project's environment.
func (e *EnvDescriber) Describe() (*EnvDescription, error) {
	var tags map[string]string
	return &EnvDescription{
		Environment:  e.env,
		Applications: e.apps,
		Tags:         tags,
	}, nil
}

// JSONString returns the stringified EnvDescription struct with json format.
func (e *EnvDescription) JSONString() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified EnvDescription struct with human readable format.
func (e *EnvDescription) HumanString() string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", e.Environment.Name)
	fmt.Fprintf(writer, "  %s\t%t\n", "Production", e.Environment.Prod)
	fmt.Fprintf(writer, "  %s\t%s\n", "Region", e.Environment.Region)
	fmt.Fprintf(writer, "  %s\t%s\n", "Account ID", e.Environment.AccountID)
	fmt.Fprintf(writer, color.Bold.Sprint("\nApplications\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	for _, app := range e.Applications {
		fmt.Fprintf(writer, "  %s\t%s\n", app.Name, app.Type)
	}
	writer.Flush()
	return b.String()
}
