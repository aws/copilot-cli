// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

var errNoAppInWorkspace = errors.New("could not find an application attached to this workspace, please run `app init` first")

// BuildSvcCmd is the top level command for service.
func BuildSvcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "svc",
		Short: "Service commands.",
		Long: `Command for working with long-running services.
A service represents an Amazon ECS service.`,
	}

	cmd.AddCommand(BuildSvcInitCmd())
	cmd.AddCommand(BuildAppListCmd())
	cmd.AddCommand(BuildSvcPackageCmd())
	cmd.AddCommand(BuildSvcDeployCmd())
	cmd.AddCommand(BuildSvcDeleteCmd())
	cmd.AddCommand(BuildSvcShowCmd())
	cmd.AddCommand(BuildAppStatusCmd())
	cmd.AddCommand(BuildAppLogsCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}
