// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildSecretCmd is the top level command for secret.
func BuildSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "secret",
		Short: `Commands for secrets.
Secrets are sensitive information that you need in your application.`,
	}

	cmd.AddCommand(buildSecretInitCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Extend,
	}
	return cmd
}
