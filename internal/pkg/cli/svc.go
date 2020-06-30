// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

var errNoAppInWorkspace = errors.New("could not find an application attached to this workspace, please run `app init` first")

// BuildSvcCmd is the top level command for service.
func BuildSvcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "svc",
		Short: `Commands for services.
Services are long-running Amazon ECS services.`,
		Long: `Commands for services.
Services are long-running Amazon ECS services.`,
	}

	cmd.AddCommand(BuildSvcInitCmd())
	cmd.AddCommand(BuildSvcListCmd())
	cmd.AddCommand(BuildSvcPackageCmd())
	cmd.AddCommand(BuildSvcDeployCmd())
	cmd.AddCommand(BuildSvcDeleteCmd())
	cmd.AddCommand(BuildSvcShowCmd())
	cmd.AddCommand(BuildSvcStatusCmd())
	cmd.AddCommand(BuildSvcLogsCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}
