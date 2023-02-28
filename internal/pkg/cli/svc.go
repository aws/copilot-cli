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
Services are long-running ECS or App Runner services.`,
		Long: `Commands for services.
Services are long-running ECS or App Runner services.`,
	}

	cmd.AddCommand(buildSvcInitCmd())
	cmd.AddCommand(buildSvcListCmd())
	cmd.AddCommand(buildSvcPackageCmd())
	cmd.AddCommand(buildSvcOverrideCmd())
	cmd.AddCommand(buildSvcDeployCmd())
	cmd.AddCommand(buildSvcDeleteCmd())
	cmd.AddCommand(buildSvcShowCmd())
	cmd.AddCommand(buildSvcStatusCmd())
	cmd.AddCommand(buildSvcLogsCmd())
	cmd.AddCommand(buildSvcExecCmd())
	cmd.AddCommand(buildSvcPauseCmd())
	cmd.AddCommand(buildSvcResumeCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}
