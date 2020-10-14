// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

const (
	docsURL = "https://aws.github.io/copilot-cli/"
)

// BuildDocsCmd builds the command for opening the documentation.
func BuildDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Open the copilot docs.",
		Long:  "Open the copilot docs.",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			var err error

			switch runtime.GOOS {
			case "linux":
				err = exec.Command("xdg-open", docsURL).Start()
			case "windows":
				err = exec.Command("rundll32", "url.dll,FileProtocolHandler", docsURL).Start()
			case "darwin":
				err = exec.Command("open", docsURL).Start()
			default:
				err = fmt.Errorf("unsupported platform")
			}
			if err != nil {
				return fmt.Errorf("open docs: %w", err)
			}

			return nil
		}),
		Annotations: map[string]string{
			"group": group.GettingStarted,
		},
	}

	cmd.SetUsageTemplate(template.Usage)

	return cmd
}
