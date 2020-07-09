// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildDeployCmd is the deploy command - which is
// an alias for app deploy.
func BuildDeployCmd() *cobra.Command {
	deployCmd := BuildSvcDeployCmd()
	deployCmd.Use = "deploy"
	deployCmd.Short = "Deploy your service."
	deployCmd.Long = `Command for deploying services to your environments.`
	deployCmd.Example = `
	Deploys a service named "frontend" to a "test" environment.
	/code $ copilot deploy --name frontend --env test`

	deployCmd.SetUsageTemplate(template.Usage)

	deployCmd.Annotations = map[string]string{
		"group": group.Release,
	}
	return deployCmd
}
