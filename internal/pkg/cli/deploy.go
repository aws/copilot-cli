// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildDeployCmd is the deploy command - which is
// an alias for app deploy.
func BuildDeployCmd() *cobra.Command {
	deployCmd := BuildSvcDeployCmd()
	deployCmd.Use = "deploy"
	deployCmd.Short = "Deploy your app."
	deployCmd.Long = `Command for deploying apps to your environments.`
	deployCmd.Example = `
	Deploys an application named "frontend" to a "test" environment.
	/code $ ecs-preview deploy --name frontend --env test`

	deployCmd.SetUsageTemplate(template.Usage)

	deployCmd.Annotations = map[string]string{
		"group": group.Release,
	}
	return deployCmd
}
