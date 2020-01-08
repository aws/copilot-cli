// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildEndpointCmd is the top level command for the storage options.
func BuildEndpointCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Endpoint commands.",
		Long:  `Command for working with URLs.`,
	}

	cmd.AddCommand(BuildEndpointCreateCmd())
	cmd.AddCommand(BuildEndpointDeleteCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Release,
	}

	return cmd
}
